package site

type securedAccessMapping struct {
	RouterAccessName string
	Group            string
}

func newSecuredAccessMapping(routerAccess, group string) securedAccessMapping {
	return securedAccessMapping{
		RouterAccessName: routerAccess,
		Group:            group,
	}
}

type securedAccessMap map[string]securedAccessMapping
