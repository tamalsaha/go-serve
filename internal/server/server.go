package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom_config "github.com/prometheus/common/config"
	"github.com/tamalsaha/go-serve/internal/tlsutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	// corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Run(addr string) error {
	cert, err := tlsutil.GenerateSelfSigned([]string{"localhost", "127.0.0.1"})
	if err != nil {
		return fmt.Errorf("generate self-signed certificate: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(formatRequestHeaders(r)))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/query", queryHandler)
	mux.HandleFunc("/external-query", externalQueryHandler)

	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	log.Printf("HTTPS server listening on %s", addr)
	return srv.Serve(tls.NewListener(ln, tlsConfig))
}

func formatRequestHeaders(r *http.Request) string {
	keys := make([]string, 0, len(r.Header))
	for key := range r.Header {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("request headers\n")
	for _, key := range keys {
		for _, value := range r.Header[key] {
			b.WriteString(key)
			b.WriteString(": ")
			b.WriteString(value)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		query = "up"
	}

	cfg := ctrl.GetConfigOrDie()

	promConfig, err := ToInternalPrometheusConfig(cfg,
		ServiceReference{
			Scheme:    "https",
			Namespace: "openshift-monitoring",
			Name:      "thanos-querier",
			Port:      9091,
		})
	if err != nil {
		http.Error(w, fmt.Sprintf("create Prometheus config: %v", err), http.StatusBadRequest)
		return
	}
	pc, err := promConfig.NewPrometheusClient()
	if err != nil {
		http.Error(w, fmt.Sprintf("create Prometheus client: %v", err), http.StatusBadRequest)
		return
	}

	res, err := getPromQueryResult(promv1.NewAPI(pc), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("query Prometheus: %v", err), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, fmt.Sprintf("encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

func externalQueryHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		query = "up"
	}

	cfg := ctrl.GetConfigOrDie()

	promConfig, err := ToExternalPrometheusConfig(cfg,
		ServiceReference{
			Scheme:    "https",
			Namespace: "openshift-monitoring",
			Name:      "thanos-querier",
			Port:      9091,
		})
	if err != nil {
		http.Error(w, fmt.Sprintf("create Prometheus config: %v", err), http.StatusBadRequest)
		return
	}
	pc, err := promConfig.NewPrometheusClient()
	if err != nil {
		http.Error(w, fmt.Sprintf("create Prometheus client: %v", err), http.StatusBadRequest)
		return
	}

	res, err := getPromQueryResult(promv1.NewAPI(pc), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("query Prometheus: %v", err), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, fmt.Sprintf("encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

func getPromQueryResult(pc promv1.API, promQuery string) (map[string]float64, error) {
	val, warn, err := pc.Query(context.Background(), promQuery, time.Now())
	if err != nil {
		return nil, err
	}
	if warn != nil {
		log.Println("warning:", warn)
	}

	metrics := strings.Split(val.String(), "\n")
	metricsMap := make(map[string]float64)

	for _, m := range metrics {
		if strings.TrimSpace(m) == "" {
			continue
		}

		parts := strings.Split(m, "=>")
		if len(parts) != 2 {
			return nil, fmt.Errorf("metrics %q is invalid for query %s", m, promQuery)
		}

		valueParts := strings.Split(parts[1], "@")
		if len(valueParts) != 2 {
			return nil, fmt.Errorf("metrics %q is invalid for query %s", m, promQuery)
		}

		numeric := strings.ReplaceAll(valueParts[0], " ", "")
		metricVal, err := strconv.ParseFloat(numeric, 64)
		if err != nil {
			return nil, err
		}

		metricsMap[parts[0]] = metricVal
	}

	return metricsMap, nil
}

type ServiceReference struct {
	Scheme    string
	Name      string
	Namespace string
	Port      int
}

func ToInternalPrometheusConfig(cfg *rest.Config, ref ServiceReference) (*PrometheusConfig, error) {
	// Load CA from the 'signing-key' secret in 'openshift-service-ca' namespace
	// and extract the 'tls.crt' key
	tokenFile := "/var/run/secrets/kubernetes.io/serviceaccount/token"

	tokenData, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("read service account token: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	cm, err := clientset.CoreV1().ConfigMaps("kube-public").Get(context.Background(), "openshift-service-ca.crt", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get openshift-service-ca.crt configmap: %w", err)
	}

	caData, ok := cm.Data["service-ca.crt"]
	if !ok {
		return nil, fmt.Errorf("service-ca.crt key not found in configmap")
	}

	// Write CA data to a temp file and set CAFile
	caFile, err := os.CreateTemp("", "openshift-service-ca-*.crt")
	if err != nil {
		return nil, fmt.Errorf("create temp ca file: %w", err)
	}
	defer caFile.Close()
	if _, err := caFile.Write([]byte(caData)); err != nil {
		return nil, fmt.Errorf("write ca data: %w", err)
	}

	return &PrometheusConfig{
		Addr:        fmt.Sprintf("https://%s.%s.svc:%d", ref.Name, ref.Namespace, ref.Port),
		BearerToken: string(tokenData),
		TLSConfig: prom_config.TLSConfig{
			CAFile:             caFile.Name(),
			ServerName:         fmt.Sprintf("%s.%s.svc", ref.Name, ref.Namespace),
			InsecureSkipVerify: false,
		},
	}, nil
}

func ToExternalPrometheusConfig(cfg *rest.Config, ref ServiceReference) (*PrometheusConfig, error) {
	// Load CA from the 'signing-key' secret in 'openshift-service-ca' namespace
	// and extract the 'tls.crt' key
	tokenFile := "/var/run/secrets/kubernetes.io/serviceaccount/token"

	tokenData, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("read service account token: %w", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}
	gvr := schema.GroupVersionResource{
		Group:    "operator.openshift.io",
		Version:  "v1",
		Resource: "ingresscontrollers",
	}
	uobj, err := dyn.Resource(gvr).Namespace("openshift-ingress-operator").Get(context.Background(), "default", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get openshift-ingress-operator default: %w", err)
	}
	domain, ok, err := unstructured.NestedString(uobj.UnstructuredContent(), "status", "domain")
	if err != nil {
		return nil, fmt.Errorf("get openshift-ingress-operator domain: %w", err)
	}
	if !ok || domain == "" {
		return nil, fmt.Errorf("openshift-ingress-operator domain not found in status")
	}
	return &PrometheusConfig{
		Addr:        fmt.Sprintf("https://%s-%s.%s", ref.Name, ref.Namespace, domain),
		BearerToken: string(tokenData),
		TLSConfig: prom_config.TLSConfig{
			CAFile: "",
			// ServerName:         fmt.Sprintf("%s.%s.svc", ref.Name, ref.Namespace),
			InsecureSkipVerify: false,
		},
	}, nil
}
