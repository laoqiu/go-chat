package gochat

import (
	"strings"
)

func Map(f func(interface{}) string, items []interface{}) []string {
	r := make([]string, len(items))
	for i, v := range items {
		r[i] = f(v)
	}
	return r
}

func splitDest(to string) (string, string) {
	if strings.Index(to, "/") == -1 {
		to = "/" + to
	}
	sp := strings.Split(to, "/")
	return sp[0], sp[1]
}

func in(v []string, c string) bool {
	for _, i := range v {
		if i == c {
			return true
		}
	}
	return false
}

func remove(v []string, c string) []string {
	r := []string{}
	for _, i := range v {
		if i != c {
			r = append(r, i)
		}
	}
	return r
}
