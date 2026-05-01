package utils

type CtxKey string

const (
	CtxKeyPrometheusCounterStore  = CtxKey("prometheus_counter_store")
	CtxKeyPromCommonLabels        = CtxKey("prom_common_labels")
	CtxKeyStartedAt               = CtxKey("started_at")
	CtxKeySharedRateLimitEnforcer = CtxKey("shared_rate_limit_enforcer")
	CtxKeyJWTSecret               = CtxKey("jwt_secret")
	CtxKeyJustIssuedJWTToken      = CtxKey("just_issued_jwt_token")
	CtxKeySessionId               = CtxKey("session_id")
	CtxKeySubjectId               = CtxKey("subject_id") // it's basically the globally unique user id
	CtxKeyRealIP                  = CtxKey("real_ip")
	CtxKeyUsername                = CtxKey("username")
)

type GlobalSharedContext struct {
	BuildVersion *BuildVersion
}
