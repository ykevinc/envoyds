package envoyds

import (
	"context"
	"github.com/BurntSushi/toml"
	"github.com/golang/protobuf/jsonpb"
	"github.com/mholt/binding"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:generate protoc pmessage.proto --go_out=.

const (
	CONTEXT_PARAMS        = "CONTEXT_PARAMS"
	CONTEXT_SERVICE       = "CONTEXT_SERVICE"
	CONTEXT_MARSHALER     = "CONTEXT_MARSHALER"
	PATH_VARIABLE_SERVICE = "service"
	PATH_VARIABLE_IP      = "ip_address"
	PATH_VARIABLE_PORT    = "port"
	HOST_TTL              = time.Minute * 10
)

type handler func(w http.ResponseWriter, r *http.Request)

type regexpRouter struct {
	routes  []*route
	context *context.Context
}

type route struct {
	pattern *regexp.Regexp
	method  string
	handler handler
}

type config struct {
	Environment string `toml:"envoyds.enviroment"`
	Port        int    `toml:"envoyds.port"`
	RedisHost   string `toml:"envoyds.redis.host"`
	RedisPort   int    `toml:"envoyds.redis.port"`
}

func ReadConfig(configPath string) *config {
	var c config
	if _, err := toml.DecodeFile(configPath, &c); err != nil {
		log.Fatal(err)
	}
	return &c
}

func NewRouter(env, redisHost string, redisPort int) (*regexpRouter, error) {
	ds, err := NewEnvoyDS(env, redisHost, redisPort)
	if err != nil {
		return nil, err
	}
	c := context.Background()
	c = context.WithValue(c, CONTEXT_SERVICE, ds)
	c = context.WithValue(c, CONTEXT_MARSHALER, &jsonpb.Marshaler{EmitDefaults: true, OrigName: true})
	r := regexpRouter{context: &c}
	r.HandleFunc(`^/v1/registration/(?P<`+PATH_VARIABLE_SERVICE+`>[^/]+)$`, http.MethodGet, getServices)
	r.HandleFunc(`^/v1/registration/repo/(?P<`+PATH_VARIABLE_SERVICE+`>[^/]+)$`, http.MethodGet, getServicesByRepo)
	r.HandleFunc(`^/v1/registration/(?P<`+PATH_VARIABLE_SERVICE+`>[^/]+)$`, http.MethodPost, registerService)
	r.HandleFunc(`^/v1/registration/(?P<`+PATH_VARIABLE_SERVICE+`>[^/]+)/(?P<`+PATH_VARIABLE_IP+`>[^/]+)$`, http.MethodDelete, deleteService)
	r.HandleFunc(`^/v1/registration/(?P<`+PATH_VARIABLE_SERVICE+`>[^/]+)/(?P<`+PATH_VARIABLE_IP+`>[^/]+)/(?P<`+PATH_VARIABLE_PORT+`>[^/]+)$`, http.MethodDelete, deleteService)
	r.HandleFunc(`^/v1/loadbalancing/(?P<`+PATH_VARIABLE_SERVICE+`>[^/]+)/(?P<`+PATH_VARIABLE_IP+`>[^/]+)$`, http.MethodPost, updateServiceWeight)
	r.HandleFunc(`^/v1/loadbalancing/(?P<`+PATH_VARIABLE_SERVICE+`>[^/]+)/(?P<`+PATH_VARIABLE_IP+`>[^/]+)/(?P<`+PATH_VARIABLE_PORT+`>[^/]+)$`, http.MethodPost, updateServiceWeight)
	return &r, nil
}

func registerService(w http.ResponseWriter, r *http.Request) {
	var (
		req ServicePostRequest
	)
	errs := binding.Bind(r, &req)
	if len(errs) > 0 {
		http.Error(w, errs.Error(), http.StatusBadRequest)
		return
	}
	serviceName := r.Context().Value(CONTEXT_PARAMS).(map[string]string)[PATH_VARIABLE_SERVICE]
	if strings.Contains(serviceName, REDIS_DELIMITER) {
		http.Error(w, "service name cannot contains " + REDIS_DELIMITER, http.StatusBadRequest)
		return
	}
	ds := r.Context().Value(CONTEXT_SERVICE).(*service)
	host := makeHost(&req)
	host.Service = serviceName
	log.Printf("registerService service=%s\n", serviceName)
	if err := ds.RegisterService(host); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func deleteService(w http.ResponseWriter, r *http.Request) {
	params := r.Context().Value(CONTEXT_PARAMS).(map[string]string)
	serviceName := params[PATH_VARIABLE_SERVICE]
	if strings.Contains(serviceName, REDIS_DELIMITER) {
		http.Error(w, "service name cannot contains "+REDIS_DELIMITER, http.StatusBadRequest)
		return
	}
	ip := params[PATH_VARIABLE_IP]
	portString := params[PATH_VARIABLE_PORT]
	if portString == "" {
		portString = "0"
	}
	ds := r.Context().Value(CONTEXT_SERVICE).(*service)
	port, err := strconv.Atoi(portString)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("deleteService service=%s ip=%s port=%d\n", serviceName, ip, port)
	c, err := ds.DeleteService(serviceName, ip, port)
	if c == 0 {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func updateServiceWeight(w http.ResponseWriter, r *http.Request) {
	var (
		req ServiceUpdateLoadBalancingRequest
	)
	errs := binding.Bind(r, &req)
	if len(errs) > 0 {
		http.Error(w, errs.Error(), http.StatusBadRequest)
		return
	}
	if req.GetLoadBalancingWeight() < 1 || req.GetLoadBalancingWeight() > 100 {
		http.Error(w, "Host weight must be an integer between 1 and 100", http.StatusBadRequest)
		return
	}
	params := r.Context().Value(CONTEXT_PARAMS).(map[string]string)
	serviceName := params[PATH_VARIABLE_SERVICE]
	if strings.Contains(serviceName, REDIS_DELIMITER) {
		http.Error(w, "service name cannot contains "+REDIS_DELIMITER, http.StatusBadRequest)
		return
	}
	ip := params[PATH_VARIABLE_IP]
	portString := params[PATH_VARIABLE_PORT]
	if portString == "" {
		portString = "0"
	}
	ds := r.Context().Value(CONTEXT_SERVICE).(*service)
	port, err := strconv.Atoi(portString)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("updateServiceWeight weight=%d\n", req.GetLoadBalancingWeight())
	c, err := ds.UpdateServiceWeight(serviceName, ip, port, req.GetLoadBalancingWeight())
	if c == 0 {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func getServices(w http.ResponseWriter, r *http.Request) {
	var (
		res ServiceGetResponse
		err error
	)
	params := r.Context().Value(CONTEXT_PARAMS).(map[string]string)
	serviceName := params[PATH_VARIABLE_SERVICE]
	if strings.Contains(serviceName, REDIS_DELIMITER) {
		http.Error(w, "service name cannot contains "+REDIS_DELIMITER, http.StatusBadRequest)
		return
	}
	ds := r.Context().Value(CONTEXT_SERVICE).(*service)
	m := r.Context().Value(CONTEXT_MARSHALER).(*jsonpb.Marshaler)
	res.Env = ds.env
	res.Hosts, err = ds.GetServicesByName(serviceName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(res.Hosts) > 0 {
		res.Service = res.Hosts[0].Service
	}
	log.Printf("getServices service=%s hosts=%d\n", serviceName, len(res.Hosts))
	if err = m.Marshal(w, &res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func getServicesByRepo(w http.ResponseWriter, r *http.Request) {
	var (
		res ServiceGetResponse
		err error
	)
	params := r.Context().Value(CONTEXT_PARAMS).(map[string]string)
	repoName := params[PATH_VARIABLE_SERVICE]
	if strings.Contains(repoName, REDIS_DELIMITER) {
		http.Error(w, "service name cannot contains "+REDIS_DELIMITER, http.StatusBadRequest)
		return
	}
	ds := r.Context().Value(CONTEXT_SERVICE).(*service)
	m := r.Context().Value(CONTEXT_MARSHALER).(*jsonpb.Marshaler)
	res.Env = ds.env
	res.Hosts, err = ds.GetServicesByRepoName(repoName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(res.Hosts) > 0 {
		res.Service = res.Hosts[0].Service
	}
	log.Printf("getServicesByRepo repoName=%s service=%s host=%d\n", repoName, res.Service, len(res.Hosts))
	if err = m.Marshal(w, &res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func makeHost(req *ServicePostRequest) *Host {
	return &Host{
		IpAddress:   req.Ip,
		LastCheckIn: strconv.Itoa(int(time.Now().UnixNano() / int64(time.Millisecond))),
		Port:        req.Port,
		Revision:    req.Revision,
		Tags:        req.Tags,
	}
}

func (h *regexpRouter) HandleFunc(pattern string, method string, handler handler) {
	h.routes = append(h.routes, &route{regexp.MustCompile(pattern), method, handler})
}

func (h *regexpRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range h.routes {
		match := route.pattern.FindStringSubmatch(r.URL.Path)
		if match != nil && route.method == r.Method {
			paramsMap := make(map[string]string)
			for i, name := range route.pattern.SubexpNames() {
				if i > 0 && i <= len(match) {
					paramsMap[name] = match[i]
				}
			}
			c := context.WithValue(*h.context, CONTEXT_PARAMS, paramsMap)
			route.handler(w, r.WithContext(c))
			return
		}
	}
	http.NotFound(w, r)
}
