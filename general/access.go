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

package general

// PublicClientHeader is an HTTP header a client (e.g. the MCP server) can
// send to signal that it forwards requests from the outside world.
// Such requests are never considered as coming from an internal network
// (no matter what their source IP is) so they are subject to auth token
// checks (if configured) and they cannot access corpora configured
// as "internal network access only".
// The header can only reduce access rights so there is no need
// to protect it against spoofing.
const PublicClientHeader = "X-Mquery-Public-Client"
