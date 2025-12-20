package utils

type CtxKey string

const (
	CtxKeyPrometheusCounterStore = CtxKey("prometheus_counter_store")
	CtxKeyPromCommonLabels = CtxKey("prom_common_labels")
)
