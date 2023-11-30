package cmdutil

import "github.com/pkg/errors"

func GetOrCreateUntypedMapFromStringMap(m map[string]interface{}, key string) (map[interface{}]interface{}, error) {
	if m[key] == nil {
		m[key] = map[interface{}]interface{}{}
	}
	typed, ok := m[key].(map[interface{}]interface{})
	if !ok {
		return nil, errors.Errorf("unexpected type for %s: %T", key, m[key])
	}
	return typed, nil
}

func GetOrCreateUntypedMap(m map[interface{}]interface{}, key string) (map[interface{}]interface{}, error) {
	if m[key] == nil {
		m[key] = map[interface{}]interface{}{}
	}
	typed, ok := m[key].(map[interface{}]interface{})
	if !ok {
		return nil, errors.Errorf("unexpected type for %s: %T", key, m[key])
	}
	return typed, nil
}
