package xmap

func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// filter return true means discarding current element
func KeysFilter[M ~map[K]V, K comparable, V any](m M, filter func(k K) bool) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		if !filter(k) {
			keys = append(keys, k)
		}
	}
	return keys
}

func Values[M ~map[K]V, K comparable, V any](m M) []V {
	vals := make([]V, 0, len(m))
	for _, v := range m {
		vals = append(vals, v)
	}
	return vals
}

// filter return true means discarding current element
func ValuesFilter[M ~map[K]V, K comparable, V any](m M, filter func(v V) bool) []V {
	vals := make([]V, 0, len(m))
	for _, v := range m {
		if !filter(v) {
			vals = append(vals, v)
		}
	}
	return vals
}

func All[M ~map[K]V, K comparable, V any](m M) ([]K, []V) {
	l := len(m)
	keys := make([]K, 0, l)
	vals := make([]V, 0, l)
	for k, v := range m {
		keys = append(keys, k)
		vals = append(vals, v)
	}

	return keys, vals
}

func Func[M ~map[K]V, K comparable, V any](m M, f func(k K, v V)) {
	for key, value := range m {
		f(key, value)
	}
}

func KVs[M ~map[K]V, K comparable, V any](m M) []any {
	r := make([]any, 0, len(m)*2)
	for k, v := range m {
		r = append(r, k, v)
	}

	return r
}

// filter returns true means discarding current element
func Filter[M ~map[K]V, K comparable, V any](m M, filter func(k K, v V) bool) M {
	r := make(M, len(m))
	for k, v := range m {
		if !filter(k, v) { // got included
			r[k] = v
		}
	}

	return r
}
