package main

import (
    "sync"
    "amoeba/lib"
)

// This ADT is a course-grained concurrent nested map structure specific
// to the amoeba server. Hence, it is not exported.
type outputMap struct {
    mu sync.Mutex
    mp map[string]map[string]lib.Output
}

func newOutputMap() *outputMap {
    var om outputMap
    om.mp = make(map[string]map[string]lib.Output)
    return &om
}

func (om *outputMap) Insert(topKey string, val map[string]lib.Output) {
    om.mu.Lock()
    defer om.mu.Unlock()
    om.mp[topKey] = val
}

func (om *outputMap) Load(topKey, botKey string) (lib.Output, bool) {
    om.mu.Lock()
    defer om.mu.Unlock()

    var empty lib.Output

    inner, ok := om.mp[topKey]
    if !ok {
        return empty, false
    }

    out, ok := inner[botKey]
    if !ok {
        return empty, false
    }

    delete(inner, botKey)

    if len(inner) == 0 {
        delete(om.mp, topKey)
    }

    return out, true
}

func (om *outputMap) TopKeys() []string {
    om.mu.Lock()
    defer om.mu.Unlock()

    keys := make([]string, len(om.mp))
    for k := range om.mp {
        keys = append(keys, k)
    }

    return keys
}

func (om *outputMap) BotKeys(topKey string) []string {
    om.mu.Lock()
    defer om.mu.Unlock()

    inner := om.mp[topKey]
    if inner == nil {
        return []string{}
    }

    keys := make([]string, len(inner))
    for k := range inner {
        keys = append(keys, k)
    }

    return keys
}
