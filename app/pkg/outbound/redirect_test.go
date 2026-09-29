package outbound

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func redirectResponse(request *http.Request, destination string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusFound,
		Header:     http.Header{"Location": []string{destination}},
		Body:       http.NoBody,
		Request:    request,
	}
}

func TestPublicRedirectRetainsStandardClientPolicy(t *testing.T) {
	client := NewClient()
	var destinationCalls int
	client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "origin.example" {
			return redirectResponse(request, "https://destination.example/finish"), nil
		}

		destinationCalls++
		if request.Header.Get("Authorization") != "" {
			t.Error("redirect forwarded credentials to a different host")
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
			Request:    request,
		}, nil
	})

	request, err := http.NewRequest(http.MethodGet, "https://origin.example/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer private-token")

	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK || destinationCalls != 1 {
		t.Fatalf("redirect did not reach destination: status=%d calls=%d", response.StatusCode, destinationCalls)
	}

	client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return redirectResponse(request, request.URL.String()), nil
	})
	response, err = client.Get("https://origin.example/loop")
	if response != nil {
		response.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("redirect loop did not stop at the standard limit: %v", err)
	}
}

func TestRedirectRejectsResolvedPrivateDestination(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	for _, destination := range []string{
		server.URL,
		strings.Replace(server.URL, "127.0.0.1", "localhost", 1),
	} {
		t.Run(destination, func(t *testing.T) {
			client := NewClient()
			transport := client.Transport
			defer transport.(*http.Transport).CloseIdleConnections()

			client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.Host == "origin.example" {
					return redirectResponse(request, destination), nil
				}

				return transport.RoundTrip(request)
			})

			response, err := client.Get("https://origin.example/start")
			if response != nil {
				response.Body.Close()
			}
			if !errors.Is(err, errPrivateDestination) {
				t.Fatalf("redirect did not reject its private destination: %v", err)
			}
		})
	}

	if calls.Load() != 0 {
		t.Fatalf("private redirect destination received %d requests", calls.Load())
	}
}
