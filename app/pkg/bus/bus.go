package bus

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

type HandlerFunc any
type Msg any

type Service interface {
	Name() string
	Category() string
	Enabled() bool
	Init()
}

var handlers = make(map[string]HandlerFunc)
var listeners = make(map[string][]HandlerFunc)
var services = make([]Service, 0)
var busLock = &sync.RWMutex{}

var frozenHandlers atomic.Pointer[map[string]HandlerFunc]
var frozenListeners atomic.Pointer[map[string][]HandlerFunc]

var shouldCount = env.IsTest()
var handlersCallCounter = make(map[string]int)
var counterLock = &sync.RWMutex{}

func Register(svc Service) {
	busLock.Lock()
	defer busLock.Unlock()

	services = append(services, svc)
}

func Reset() {
	busLock.Lock()
	defer busLock.Unlock()

	frozenHandlers.Store(nil)
	frozenListeners.Store(nil)

	services = make([]Service, 0)
	handlers = make(map[string]HandlerFunc)
	listeners = make(map[string][]HandlerFunc)

	handlersCallCounter = make(map[string]int)
}

func Init(forcedServices ...Service) []Service {
	initializedServices := make([]Service, 0)
	for _, svc := range forcedServices {
		initializedServices = append(initializedServices, svc)
		svc.Init()
	}

	for _, svc := range services {
		if svc.Enabled() {
			initializedServices = append(initializedServices, svc)
			svc.Init()
		}
	}
	return initializedServices
}

func Freeze() {
	busLock.RLock()
	defer busLock.RUnlock()

	h := make(map[string]HandlerFunc, len(handlers))
	for k, v := range handlers {
		h[k] = v
	}
	frozenHandlers.Store(&h)

	l := make(map[string][]HandlerFunc, len(listeners))
	for k, v := range listeners {
		cpy := make([]HandlerFunc, len(v))
		copy(cpy, v)
		l[k] = cpy
	}
	frozenListeners.Store(&l)
}

func AddHandler(handler HandlerFunc) {
	busLock.Lock()
	defer busLock.Unlock()

	handlerType := reflect.TypeOf(handler)
	elem := handlerType.In(1).Elem()
	handlers[keyForElement(elem)] = handler
}

func AddListener(handler HandlerFunc) {
	busLock.Lock()
	defer busLock.Unlock()

	handlerType := reflect.TypeOf(handler)
	elem := handlerType.In(1).Elem()
	eventName := keyForElement(elem)
	_, exists := listeners[eventName]
	if !exists {
		listeners[eventName] = make([]HandlerFunc, 0)
	}
	listeners[eventName] = append(listeners[eventName], handler)
}

func MustDispatch(ctx context.Context, msgs ...Msg) {
	err := Dispatch(ctx, msgs...)
	if err != nil {
		panic(err)
	}
}

func Dispatch(ctx context.Context, msgs ...Msg) error {
	if len(msgs) == 0 {
		return nil
	}

	h := frozenHandlers.Load()
	if h != nil {
		return dispatchWithHandlers(ctx, *h, msgs)
	}

	busLock.RLock()
	defer busLock.RUnlock()
	return dispatchWithHandlers(ctx, handlers, msgs)
}

func dispatchWithHandlers(ctx context.Context, h map[string]HandlerFunc, msgs []Msg) error {
	for _, msg := range msgs {
		key := getKey(msg)
		handler := h[key]
		if handler == nil {
			panic(fmt.Errorf("could not find handler for '%s'.", key))
		}

		var params = []reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(msg),
		}

		if shouldCount {
			counterLock.Lock()
			handlersCallCounter[key]++
			counterLock.Unlock()
		}

		ret := reflect.ValueOf(handler).Call(params)
		if err := ret[0].Interface(); err != nil {
			return err.(error)
		}
	}

	return nil
}

func Publish(ctx context.Context, msgs ...Msg) {
	if len(msgs) == 0 {
		return
	}

	l := frozenListeners.Load()
	if l != nil {
		publishWithListeners(ctx, *l, msgs)
		return
	}

	busLock.RLock()
	defer busLock.RUnlock()
	publishWithListeners(ctx, listeners, msgs)
}

func publishWithListeners(ctx context.Context, l map[string][]HandlerFunc, msgs []Msg) {
	for _, msg := range msgs {
		key := getKey(msg)
		msgListeners := l[key]

		var params = []reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(msg),
		}

		for _, msgListener := range msgListeners {
			ret := reflect.ValueOf(msgListener).Call(params)
			if len(ret) > 0 {
				if err, isErr := ret[0].Interface().(error); isErr {
					Publish(ctx, &cmd.LogError{
						Err: errors.Wrap(err, "failed to execute msg '%s'", key),
					})
				}
			}
		}
	}
}

func GetCallCount(msg Msg) int {
	key := getKey(msg)
	return handlersCallCounter[key]
}

func getKey(msg Msg) string {
	typeof := reflect.TypeOf(msg)
	if typeof.Kind() != reflect.Ptr {
		panic(fmt.Errorf("'%s' is not a pointer", keyForElement(typeof)))
	}

	elem := typeof.Elem()
	return keyForElement(elem)
}

func keyForElement(t reflect.Type) string {
	msgTypeName := t.Name()
	pkgPath := t.PkgPath()
	return pkgPath + "." + msgTypeName
}
