package utils

import "fmt"

func GetBytes(key interface{}) []byte {
	return []byte(fmt.Sprintf("%v", key))
}
