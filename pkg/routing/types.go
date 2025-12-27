package routing

type Router interface {
	AddRoute(cidr string, value interface{}) error
	GetRoute(ip string) (interface{}, error)
}
