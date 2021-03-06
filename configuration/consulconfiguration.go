/*
 * === This file is part of ALICE O² ===
 *
 * Copyright 2018 CERN and copyright holders of ALICE O².
 * Author: Teo Mrnjavac <teo.mrnjavac@cern.ch>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 * In applying this license CERN does not waive the privileges and
 * immunities granted to it by virtue of its status as an
 * Intergovernmental Organization or submit itself to any jurisdiction.
 */

package configuration

import (
	"github.com/hashicorp/consul/api"
	"strings"
)

type ConsulConfiguration struct {
	uri string
	kv  *api.KV
}

func newConsulConfiguration(uri string) (cc *ConsulConfiguration, err error) {
	cfg := api.DefaultConfig()
	cfg.Address = uri
	cli, err := api.NewClient(cfg)
	if err != nil {
		return
	}
	cc = &ConsulConfiguration{
		uri: uri,
		kv: cli.KV(),
	}
	return
}

func (cc *ConsulConfiguration) Get(key string) (value string, err error) {
	kvp, _, err := cc.kv.Get(formatKey(key), nil)
	if err != nil {
		return
	}
	value = string(kvp.Value[:])
	return
}

func (cc *ConsulConfiguration) GetRecursive(key string) (value Map, err error) {
	requestKey := formatKey(key)
	kvps, _, err := cc.kv.List(requestKey, nil)
	if err != nil {
		return
	}
	for _, kvp := range kvps {
		kvp.Key = stripRequestKey(requestKey, kvp.Key)
	}
	return mapify(kvps), nil
}

func (cc *ConsulConfiguration) Put(key string, value string) (err error) {
	kvp := &api.KVPair{Key: formatKey(key), Value: []byte(value)}
	_, err = cc.kv.Put(kvp, nil)
	return
}

func (cc *ConsulConfiguration) Exists(key string) (exists bool, err error) {
	kvp, _, err := cc.kv.Get(formatKey(key), nil)
	if err != nil {
		return
	}
	exists = kvp != nil
	return
}

func formatKey(key string) (consulKey string) {
	// Trim leading slashes
	consulKey = strings.TrimLeft(key, "/")
	return
}

func stripRequestKey(requestKey string, responseKey string) string {
	// The request key is prefixed to the response keys, this strips that from it.
	return strings.TrimPrefix(responseKey, requestKey)
}

func mapify(kvps api.KVPairs) Map {
	// Our output Map (=map[string]Item)
	m := make(Map)

	// We accumulate a partial map here, by stripping the leftmost
	// part of the key as prefix, and associating it with a slice
	// of KVPairs for that prefix.
	prefixSet := make(map[string]api.KVPairs)

	for _, kvp := range kvps {
		i := strings.IndexByte(kvp.Key, '/')
		if i == 0 {
			// Looks like the key starts with "/". This should never
			// happen but we try to recover from it by trimming leading
			// slashes and checking whether we still have a key.
			kvp.Key = strings.TrimLeft(kvp.Key, "/")
			if len(kvp.Key) == 0 {
				continue //Nothing to do here with an empty key
			}
			i = strings.IndexByte(kvp.Key, '/')
		}
		if i == -1 {
			// This key has no separator, so it has no prefix, so it's
			// a leaf in our tree.
			// We convert its value into a configuration.String and
			// we're done.
			m[kvp.Key] = String(kvp.Value)
		} else {
			// A separator was found. If the Consul output is in any way
			// legit, i cannot be 0
			prefix := kvp.Key[:i]
			kvp.Key = kvp.Key[i+1:]
			if prefixSet[prefix] == nil {
				prefixSet[prefix] = make(api.KVPairs, 0)
			}
			prefixSet[prefix] = append(prefixSet[prefix], kvp)
		}
	}
	for prefix, kvpairslist := range prefixSet {
		m[prefix] = mapify(kvpairslist)
	}
	return m
}