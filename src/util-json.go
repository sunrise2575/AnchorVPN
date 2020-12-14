package main

import "github.com/tidwall/gjson"

type myJSONType struct {
	content string
}

var (
	jsn myJSONType
)

func (j *myJSONType) register(filePath string) {
	j.content = supportFile2Str(filePath)
}

func (j *myJSONType) get(query string) string {
	return gjson.Get(j.content, query).String()
}

func (j *myJSONType) getNestedArray(query string, result chan string, subQuery ...string) {
	defer close(result)
	tmp := gjson.Get(j.content, query)
	tmp.ForEach(func(key, value gjson.Result) bool {
		for _, q := range subQuery {
			a := gjson.Get(value.String(), q).String()
			result <- a
		}
		return true
	})
}

func (j *myJSONType) getSimpleArray(input string, result chan string) {
	defer close(result)
	input = `{ "temporary" : ` + input + `}`
	tmp := gjson.Get(input, "temporary")
	tmp.ForEach(func(key, value gjson.Result) bool {
		result <- value.String()
		return true
	})
}
