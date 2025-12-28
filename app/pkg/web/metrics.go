package web

import (
	"net/http"
	"net/http/pprof"
	"runtime"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func newMetricsServer(address string) *http.Server {
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)
	mux := httprouter.New()
	mux.Handler("GET", "/metrics", promhttp.Handler())

	mux.HandlerFunc("GET", "/debug/pprof/", pprof.Index)
	mux.HandlerFunc("GET", "/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandlerFunc("GET", "/debug/pprof/profile", pprof.Profile)
	mux.HandlerFunc("GET", "/debug/pprof/symbol", pprof.Symbol)
	mux.HandlerFunc("GET", "/debug/pprof/trace", pprof.Trace)
	mux.Handler("GET", "/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handler("GET", "/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handler("GET", "/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	mux.Handler("GET", "/debug/pprof/block", pprof.Handler("block"))
	mux.Handler("GET", "/debug/pprof/mutex", pprof.Handler("mutex"))

	return &http.Server{
		ReadTimeout:  env.Config.HTTP.ReadTimeout,
		WriteTimeout: env.Config.HTTP.WriteTimeout,
		IdleTimeout:  env.Config.HTTP.IdleTimeout,
		Addr:         address,
		Handler:      mux,
	}
}
