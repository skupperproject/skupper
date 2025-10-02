package site

type securedAccessMapping struct {
	RotuerAccessName string
	Group            string
}

func newSecuredAccessMapping(routerAccess, group string) securedAccessMapping {
	return securedAccessMapping{
		RotuerAccessName: routerAccess,
		Group:            group,
	}
}

type securedAccessMap map[string]securedAccessMapping
