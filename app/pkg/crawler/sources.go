package crawler

import (
	"net"
	"sync/atomic"
)

type Source struct {
	Name     string
	URLs     []string
	prefixes atomic.Value
}

func (s *Source) Prefixes() []*net.IPNet {
	v := s.prefixes.Load()
	if v == nil {
		return nil
	}
	return v.([]*net.IPNet)
}

func (s *Source) SetPrefixes(p []*net.IPNet) {
	s.prefixes.Store(p)
}

func (s *Source) Contains(ip net.IP) bool {
	prefixes := s.Prefixes()
	if prefixes == nil {
		return false
	}
	for _, prefix := range prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

var Sources = []*Source{
	{
		Name: "google",
		URLs: []string{
			"https://developers.google.com/static/search/apis/ipranges/googlebot.json",
			"https://developers.google.com/static/search/apis/ipranges/special-crawlers.json",
		},
	},
	{
		Name: "bing",
		URLs: []string{
			"https://www.bing.com/toolbox/bingbot.json",
		},
	},
}
