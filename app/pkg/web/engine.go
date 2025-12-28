package web

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	stdLog "log"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/worker"
	"github.com/julienschmidt/httprouter"
	cache "github.com/patrickmn/go-cache"
)

var (
	cspBase           = "base-uri 'self'"
	cspDefault        = "default-src 'self'"
	cspStyle          = "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://*.paddle.com %[2]s"
	cspScript         = "script-src 'self' 'unsafe-inline' 'nonce-%[1]s' https://ep1.adtrafficquality.google https://www.google-analytics.com https://*.paddle.com https://*.googletagmanager.com https://pagead2.googlesyndication.com https://static.cloudflareinsights.com https://*.cloudflare.com %[2]s"
	cspFont           = "font-src 'self' https://fonts.gstatic.com data: %[2]s"
	cspImage          = "img-src 'self' https: data: https://*.google-analytics.com https://*.analytics.google.com https://*.googletagmanager.com https://*.g.doubleclick.net %[2]s"
	cspObject         = "object-src 'none'"
	cspFrame          = "frame-src 'self' https://*.paddle.com https://td.doubleclick.net https://www.googletagmanager.com https://www.youtube.com/ https://vk.com/ https://vkvideo.ru/"
	cspMedia          = "media-src 'none'"
	cspConnect        = "connect-src 'self' https://*.google-analytics.com https://*.analytics.google.com https://*.googletagmanager.com https://*.g.doubleclick.net https://cloudflareinsights.com https://*.cloudflare.com %[2]s"
	cspFrameAncestors = "frame-ancestors 'none'"

	CspPolicyTemplate = fmt.Sprintf("%s; %s; %s; %s; %s; %s; %s; %s; %s; %s; %s", cspBase, cspDefault, cspStyle, cspScript, cspImage, cspFont, cspObject, cspMedia, cspConnect, cspFrame, cspFrameAncestors)
)

type notFoundHandler struct {
	engine  *Engine
	handler HandlerFunc
}

func (h *notFoundHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	ctx := NewContext(h.engine, req, res, nil)
	_ = h.handler(ctx)
}

type HandlerFunc func(*Context) error

type MiddlewareFunc func(HandlerFunc) HandlerFunc

type Engine struct {
	context.Context
	mux           *httprouter.Router
	renderer      *Renderer
	binder        *DefaultBinder
	middlewares   []MiddlewareFunc
	worker        worker.Worker
	webServer     *http.Server
	metricsServer *http.Server
	cache         *cache.Cache
}

func New() *Engine {
	ctx := context.Background()
	ctx = log.WithProperties(ctx, dto.Props{
		log.PropertyKeyContextID: rand.String(32),
		log.PropertyKeyTag:       "WEB",
	})

	mux := httprouter.New()
	mux.SaveMatchedRoutePath = true

	router := &Engine{
		Context:     ctx,
		mux:         mux,
		renderer:    NewRenderer(),
		binder:      NewDefaultBinder(),
		middlewares: make([]MiddlewareFunc, 0),
		worker:      worker.New(),
		cache:       cache.New(5*time.Minute, 10*time.Minute),
	}

	return router
}

func (e *Engine) Start(address string) {
	log.Info(e, "Application is starting")
	log.Infof(e, "Version: @{Version}", dto.Props{
		"Version": env.Version(),
	})
	log.Infof(e, "Environment: @{Env}", dto.Props{
		"Env": env.Config.Environment,
	})

	var (
		certFilePath = ""
		keyFilePath  = ""
	)

	if env.Config.TLS.Certificate != "" {
		certFilePath = env.Etc(env.Config.TLS.Certificate)
		keyFilePath = env.Etc(env.Config.TLS.CertificateKey)
	}

	stdLog.SetOutput(io.Discard)
	e.webServer = &http.Server{
		ReadTimeout:  env.Config.HTTP.ReadTimeout,
		WriteTimeout: env.Config.HTTP.WriteTimeout,
		IdleTimeout:  env.Config.HTTP.IdleTimeout,
		Addr:         address,
		Handler:      e.mux,
		TLSConfig:    getDefaultTLSConfig(env.Config.TLS.Automatic),
	}

	for i := 0; i < runtime.NumCPU(); i++ {
		go e.Worker().Run(strconv.Itoa(i))
	}

	if env.Config.Metrics.Enabled {
		metricsAddress := ":" + env.Config.Metrics.Port
		e.metricsServer = newMetricsServer(metricsAddress)
		go func() {
			err := e.metricsServer.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				panic(errors.Wrap(err, "failed to start metrics server"))
			}
		}()
	}

	var (
		err         error
		certManager *CertificateManager
	)

	if env.Config.TLS.Automatic {
		certManager, err = NewCertificateManager(e, certFilePath, keyFilePath)
		if err != nil {
			panic(errors.Wrap(err, "failed to initialize CertificateManager"))
		}

		e.webServer.TLSConfig.GetCertificate = certManager.GetCertificate
		log.Infof(e, "https (auto tls) server started on @{Address}", dto.Props{"Address": address})
		go certManager.StartHTTPServer()
		err = e.webServer.ListenAndServeTLS("", "")
	} else if certFilePath == "" && keyFilePath == "" {
		log.Infof(e, "http server started on @{Address}", dto.Props{"Address": address})
		err = e.webServer.ListenAndServe()
	} else {
		log.Infof(e, "https server started on @{Address}", dto.Props{"Address": address})
		err = e.webServer.ListenAndServeTLS(certFilePath, keyFilePath)
	}

	if err != nil && err != http.ErrServerClosed {
		panic(errors.Wrap(err, "failed to start server"))
	}
}

func (e *Engine) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if e.metricsServer != nil {
		log.Info(e, "metrics server is shutting down")
		if err := e.metricsServer.Shutdown(ctx); err != nil {
			return errors.Wrap(err, "failed to shutdown metrics server")
		}
		log.Info(e, "metrics server has shutdown")
	}

	if e.webServer != nil {
		log.Info(e, "web server is shutting down")
		if err := e.webServer.Shutdown(ctx); err != nil {
			return errors.Wrap(err, "failed to shutdown web server")
		}
		log.Info(e, "web server has shutdown")
	}

	if e.worker != nil {
		log.Info(e, "worker is shutting down")
		if err := e.worker.Shutdown(ctx); err != nil {
			return errors.Wrap(err, "failed to shutdown worker")
		}
		log.Info(e, "worker has shutdown")
	}

	return nil
}

func (e *Engine) Cache() *cache.Cache {
	return e.cache
}

func (e *Engine) Worker() worker.Worker {
	return e.worker
}

func (e *Engine) Group() *Group {
	g := &Group{
		engine:      e,
		middlewares: e.middlewares,
	}
	return g
}

func (e *Engine) Use(middleware MiddlewareFunc) {
	if middleware == nil {
		return
	}

	e.middlewares = append(e.middlewares, middleware)
}

func (e *Engine) Get(path string, handler HandlerFunc) {
	e.mux.Handle("GET", path, e.handle(e.middlewares, handler))
}

func (e *Engine) Post(path string, handler HandlerFunc) {
	e.mux.Handle("POST", path, e.handle(e.middlewares, handler))
}

func (e *Engine) Put(path string, handler HandlerFunc) {
	e.mux.Handle("PUT", path, e.handle(e.middlewares, handler))
}

func (e *Engine) Delete(path string, handler HandlerFunc) {
	e.mux.Handle("DELETE", path, e.handle(e.middlewares, handler))
}

func (e *Engine) NotFound(handler HandlerFunc) {
	e.mux.NotFound = &notFoundHandler{
		engine:  e,
		handler: handler,
	}
}

func (e *Engine) handle(middlewares []MiddlewareFunc, handler HandlerFunc) httprouter.Handle {
	next := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		next = middlewares[i](next)
	}
	var h = func(res http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		params := make(StringMap)
		for _, p := range ps {
			params[p.Key] = p.Value
		}
		ctx := NewContext(e, req, res, params)
		_ = next(ctx)
	}
	return h
}

type Group struct {
	engine      *Engine
	middlewares []MiddlewareFunc
}

func (g *Group) Group() *Group {
	g2 := &Group{
		engine:      g.engine,
		middlewares: g.middlewares,
	}
	return g2
}

func (g *Group) Use(middleware MiddlewareFunc) {
	g.middlewares = append(g.middlewares, middleware)
}

func (g *Group) Get(path string, handler HandlerFunc) {
	g.engine.mux.Handle("GET", path, g.engine.handle(g.middlewares, handler))
}

func (g *Group) Post(path string, handler HandlerFunc) {
	g.engine.mux.Handle("POST", path, g.engine.handle(g.middlewares, handler))
}

func (g *Group) Put(path string, handler HandlerFunc) {
	g.engine.mux.Handle("PUT", path, g.engine.handle(g.middlewares, handler))
}

func (g *Group) Delete(path string, handler HandlerFunc) {
	g.engine.mux.Handle("DELETE", path, g.engine.handle(g.middlewares, handler))
}

func (g *Group) Static(prefix, root string) {
	subFS, err := fs.Sub(assets.FS, root)
	if err != nil {
		panic(fmt.Sprintf("Failed to create sub filesystem for '%s': %v", root, err))
	}

	fileServer := http.FileServer(http.FS(subFS))

	h := func(c *Context) error {
		filePath := c.Param("filepath")
		filePath = strings.TrimPrefix(filePath, "/")

		_, err := fs.Stat(subFS, filePath)
		if err != nil {
			c.Response.Header().Set("Cache-Control", "no-cache, no-store")
			return c.NotFound()
		}

		c.Request.instance.URL.Path = "/" + filePath
		fileServer.ServeHTTP(&c.Response, c.Request.instance)
		return nil
	}

	g.engine.mux.Handle("GET", prefix, g.engine.handle(g.middlewares, h))
}

func ParseCookie(s string) *http.Cookie {
	if s == "" {
		return nil
	}
	return (&http.Response{Header: http.Header{"Set-Cookie": {s}}}).Cookies()[0]
}
