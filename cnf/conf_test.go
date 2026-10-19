// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of MQUERY.
//
//  MQUERY is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  MQUERY is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with MQUERY.  If not, see <https://www.gnu.org/licenses/>.

package cnf

import (
	"mquery/general"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInternalRequest(t *testing.T) {
	const listenAddr = "10.0.0.1"
	strict := &AuthConf{
		LocalNetworks: []string{"10.0.0.0/24"},
		KnownProxies:  []string{"10.0.0.5"},
	}
	withFwdHeaders := &AuthConf{
		LocalNetworks:               []string{"10.0.0.0/24"},
		KnownProxies:                []string{"10.0.0.5"},
		ProxiesSetForwardingHeaders: true,
	}

	tests := []struct {
		name     string
		conf     *AuthConf
		remote   string
		headers  map[string]string
		expected bool
	}{
		{"local direct", strict, "10.0.0.7:1234", nil, true},
		{"outside network", strict, "192.168.1.1:1234", nil, false},
		{"known proxy, strict", strict, "10.0.0.5:1234", nil, false},
		{"known proxy, strict, fwd header", strict, "10.0.0.5:1234",
			map[string]string{"X-Forwarded-For": "1.2.3.4"}, false},
		{"known proxy, no fwd header", withFwdHeaders, "10.0.0.5:1234", nil, true},
		{"known proxy, X-Forwarded-For", withFwdHeaders, "10.0.0.5:1234",
			map[string]string{"X-Forwarded-For": "1.2.3.4"}, false},
		{"known proxy, X-Real-IP", withFwdHeaders, "10.0.0.5:1234",
			map[string]string{"X-Real-IP": "1.2.3.4"}, false},
		{"known proxy, Forwarded", withFwdHeaders, "10.0.0.5:1234",
			map[string]string{"Forwarded": "for=1.2.3.4"}, false},
		{"local direct, fwd header ignored", withFwdHeaders, "10.0.0.7:1234",
			map[string]string{"X-Forwarded-For": "1.2.3.4"}, true},
		{"public client header", withFwdHeaders, "10.0.0.7:1234",
			map[string]string{general.PublicClientHeader: "1"}, false},
		{"nil conf, listen address", nil, "10.0.0.1:1234", nil, true},
		{"nil conf, other address", nil, "10.0.0.7:1234", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remote
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			assert.Equal(t, tt.expected, tt.conf.IsInternalRequest(req, listenAddr))
		})
	}
}
