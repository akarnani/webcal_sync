package main

import "encoding/json"

func jsonEscape(s string) string {
	o, err := json.Marshal(s)
	if err != nil {
		// if it can't be json encoded just return the original
		return s
	}

	str := string(o)
	return str[1 : len(str)-1] // strip the leading/trailing quotes
}
