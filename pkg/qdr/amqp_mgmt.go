package qdr

import (
	"context"
	"crypto/tls"
	"fmt"
	amqp "github.com/interconnectedcloud/go-amqp"
	"strings"
	"time"
)

type Agent struct {
	connection *amqp.Client
	sender     *amqp.Sender
	anonymous  *amqp.Sender
	receiver   *amqp.Receiver
	local      *Router
	closed     bool
}

type Router struct {
	Id          string
	Address     string
	Edge        bool
	SiteId      string
	ConnectedTo []string
}

type Record map[string]interface{}

func (r Record) AsString(field string) string {
	if value, ok := r[field].(string); ok {
		return value
	} else {
		return ""
	}
}

func (r Record) AsBool(field string) bool {
	if value, ok := r[field].(bool); ok {
		return value
	} else {
		return false
	}
}

func (r Record) AsInt(field string) int {
	value, _ := AsInt(r[field])
	return value
}

func (r Record) AsUint64(field string) uint64 {
	value, _ := AsUint64(r[field])
	return value
}

func (r Record) AsRecord(field string) Record {
	if value, ok := r[field].(map[string]interface{}); ok {
		return value
	} else {
		return nil
	}
}

func asConnection(record Record) Connection {
	return Connection{
		Role:       record.AsString("role"),
		Container:  record.AsString("container"),
		Host:       record.AsString("host"),
		OperStatus: record.AsString("operStatus"),
		Dir:        record.AsString("dir"),
		Active:     record.AsBool("active"),
	}
}

func asRouterNode(record Record) RouterNode {
	return RouterNode{
		Id:      record.AsString("id"),
		Name:    record.AsString("name"),
		Address: record.AsString("address"),
		NextHop: record.AsString("nextHop"),
	}
}

func asRouter(record Record) *Router {
	r := Router{
		Id:     record.AsString("id"),
		SiteId: record.AsString("metadata"),
	}
	if record.AsString("mode") == "edge" {
		r.Edge = true
	} else {
		r.Edge = false
	}
	r.Address = getRouterAgentAddress(r.Id, r.Edge)
	return &r
}

func (node *RouterNode) asRouter() *Router {
	return &Router{
		Id: node.Id,
		//SiteId ???
		Address: node.Address,
		Edge:    false, /*RouterNode is always an interior*/
	}
}

type AgentPool struct {
	url    string
	config *tls.Config
	pool   chan *Agent
}

func NewAgentPool(url string, config *tls.Config) *AgentPool {
	return &AgentPool{
		url:    url,
		config: config,
		pool:   make(chan *Agent, 10),
	}
}

func (p *AgentPool) Get() (*Agent, error) {
	var a *Agent
	var err error
	select {
	case a = <-p.pool:
	default:
		a, err = Connect(p.url, p.config)
	}
	return a, err
}

func (p *AgentPool) Put(a *Agent) {
	if !a.closed {
		select {
		case p.pool <- a:
		default:
			a.Close()
		}
	}
}

func Connect(url string, config *tls.Config) (*Agent, error) {
	var connection *amqp.Client
	var err error
	if config == nil {
		connection, err = amqp.Dial(url, amqp.ConnMaxFrameSize(4294967295))
	} else {
		connection, err = amqp.Dial(url, amqp.ConnSASLExternal(), amqp.ConnMaxFrameSize(4294967295), amqp.ConnTLSConfig(config))
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to create connection: %s", err)
	}
	session, err := connection.NewSession()
	if err != nil {
		return nil, fmt.Errorf("Failed to create session: %s", err)
	}

	receiver, err := session.NewReceiver(
		amqp.LinkSourceAddress(""),
		amqp.LinkAddressDynamic(),
		amqp.LinkCredit(10),
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to create receiver: %s", err)
	}
	sender, err := session.NewSender(
		amqp.LinkTargetAddress("$management"),
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to create sender: %s", err)
	}
	anonymous, err := session.NewSender()
	if err != nil {
		return nil, fmt.Errorf("Failed to create anonymous sender: %s", err)
	}
	a := &Agent{
		connection: connection,
		sender:     sender,
		anonymous:  anonymous,
		receiver:   receiver,
	}
	a.local, err = a.getLocalRouter()
	if err != nil {
		return a, fmt.Errorf("Failed to lookup local router details: %s", err)
	}
	return a, nil
}

func (a *Agent) Close() error {
	a.closed = true
	return a.connection.Close()
}

func isOk(code int) bool {
	return code >= 200 && code < 300
}

func cleanup(input interface{}) interface{} {
	switch input.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for k, v := range input.(map[interface{}]interface{}) {
			m[k.(string)] = cleanup(v)
		}
		return m
	case map[string]interface{}:
		m := input.(map[string]interface{})
		for k, v := range m {
			m[k] = cleanup(v)
		}
		return m
	default:
		return input
	}
}

func makeRecord(fields []string, values []interface{}) Record {
	record := Record{}
	for i, name := range fields {
		record[name] = cleanup(values[i])
	}
	return record
}

func stringify(items []interface{}) []string {
	s := make([]string, len(items))
	for i := range items {
		s[i] = fmt.Sprintf("%v", items[i])
	}
	return s
}

func getRouterAgentAddress(id string, edge bool) string {
	if edge {
		return "amqp:/_edge/" + id + "/$management"
	} else {
		return "amqp:/_topo/0/" + id + "/$management"
	}
}

func getRouterAddress(id string, edge bool) string {
	if edge {
		return "amqp:/_edge/" + id
	} else {
		return "amqp:/_topo/0/" + id
	}
}

func (a *Agent) Query(typename string, attributes []string) ([]Record, error) {
	return a.QueryRouterNode(typename, attributes, nil)
}

func (a *Agent) QueryRouterNode(typename string, attributes []string, node *RouterNode) ([]Record, error) {
	var address string
	if node != nil {
		address = node.Address
	}
	return a.QueryByAgentAddress(typename, attributes, address)
}

func AsInt(value interface{}) (int, bool) {
	switch value.(type) {
	case uint8:
		return int(value.(uint8)), true
	case uint16:
		return int(value.(uint16)), true
	case uint32:
		return int(value.(uint32)), true
	case uint64:
		return int(value.(uint64)), true
	case int8:
		return int(value.(int8)), true
	case int16:
		return int(value.(int16)), true
	case int32:
		return int(value.(int32)), true
	case int64:
		return int(value.(int64)), true
	case int:
		return value.(int), true
	default:
		return 0, false
	}
}

func AsUint64(value interface{}) (uint64, bool) {
	switch value.(type) {
	case uint8:
		return uint64(value.(uint8)), true
	case uint16:
		return uint64(value.(uint16)), true
	case uint32:
		return uint64(value.(uint32)), true
	case uint64:
		return value.(uint64), true
	case int8:
		return uint64(value.(int8)), true
	case int16:
		return uint64(value.(int16)), true
	case int32:
		return uint64(value.(int32)), true
	case int64:
		return uint64(value.(int64)), true
	case int:
		return uint64(value.(int)), true
	default:
		return 0, false
	}
}

func (a *Agent) QueryByAgentAddress(typename string, attributes []string, agent string) ([]Record, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	var request amqp.Message
	var properties amqp.MessageProperties
	properties.ReplyTo = a.receiver.Address()
	properties.CorrelationID = uint64(1)
	request.Properties = &properties
	request.ApplicationProperties = make(map[string]interface{})
	request.ApplicationProperties["operation"] = "QUERY"
	request.ApplicationProperties["entityType"] = typename
	var body = make(map[string]interface{})
	body["attributeNames"] = attributes
	request.Value = body

	var err error
	if agent == "" {
		err = a.sender.Send(ctx, &request)
	} else {
		request.Properties.To = agent
		err = a.anonymous.Send(ctx, &request)
	}
	if err != nil {
		a.Close()
		return nil, fmt.Errorf("Could not send request: %s", err)
	}

	response, err := a.receiver.Receive(ctx)
	if err != nil {
		a.Close()
		return nil, fmt.Errorf("Failed to receive reponse: %s", err)
	}
	response.Accept()
	if status, ok := AsInt(response.ApplicationProperties["statusCode"]); ok && isOk(status) {
		if top, ok := response.Value.(map[string]interface{}); ok {
			records := []Record{}
			fields := stringify(top["attributeNames"].([]interface{}))
			results := top["results"].([]interface{})
			for _, r := range results {
				o := r.([]interface{})
				records = append(records, makeRecord(fields, o))
			}
			return records, nil
		} else {
			return nil, fmt.Errorf("Bad response: %s", response.Value)
		}
	} else {
		return nil, fmt.Errorf("Query failed with: %s", response.ApplicationProperties["statusDescription"])
	}
}

type Query struct {
	typename   string
	attributes []string
	agent      string
}

func queryAllAgents(typename string, agents []string) []Query {
	queries := make([]Query, len(agents))
	for i, a := range agents {
		queries[i].typename = typename
		queries[i].attributes = []string{}
		queries[i].agent = a
	}
	return queries
}

func queryAllTypes(typenames []string, agent string) []Query {
	queries := make([]Query, len(typenames))
	for i, t := range typenames {
		queries[i].typename = t
		queries[i].attributes = []string{}
		queries[i].agent = agent
	}
	return queries
}

func queryAllAgentsForAllTypes(typenames []string, agents []string) []Query {
	queries := make([]Query, len(agents)*len(typenames))
	i := 0
	for _, t := range typenames {
		for _, a := range agents {
			queries[i].typename = t
			queries[i].attributes = []string{}
			queries[i].agent = a
			i++
		}
	}
	return queries
}

func (a *Agent) BatchQuery(queries []Query) ([][]Record, error) {
	fmt.Printf("BatchQuery(%v)\n", queries)
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	batchResults := make([][]Record, len(queries))
	for i, q := range queries {
		var request amqp.Message
		var properties amqp.MessageProperties
		properties.ReplyTo = a.receiver.Address()
		properties.CorrelationID = uint64(i)
		request.Properties = &properties
		request.ApplicationProperties = make(map[string]interface{})
		request.ApplicationProperties["operation"] = "QUERY"
		request.ApplicationProperties["entityType"] = q.typename
		var body = make(map[string]interface{})
		body["attributeNames"] = q.attributes
		request.Value = body

		var err error
		if q.agent == "" {
			err = a.sender.Send(ctx, &request)
		} else {
			request.Properties.To = q.agent
			err = a.anonymous.Send(ctx, &request)
		}
		if err != nil {
			a.Close()
			return nil, fmt.Errorf("Could not send request: %s", err)
		}
	}
	errors := []string{}
	for i := 0; i < len(queries); i++ {
		fmt.Printf("Waiting for response %d of %d\n", (i + 1), len(queries))
		response, err := a.receiver.Receive(ctx)
		if err != nil {
			a.Close()
			return nil, fmt.Errorf("Failed to receive reponse: %s", err)
		}
		response.Accept()
		responseIndex, ok := response.Properties.CorrelationID.(uint64)
		if !ok {
			errors = append(errors, fmt.Sprintf("Could not get correct correlation id from response: %#v (%T)", response.Properties.CorrelationID, response.Properties.CorrelationID))
		} else {
			if status, ok := AsInt(response.ApplicationProperties["statusCode"]); ok && isOk(status) {
				if top, ok := response.Value.(map[string]interface{}); ok {
					records := []Record{}
					fields := stringify(top["attributeNames"].([]interface{}))
					results := top["results"].([]interface{})
					for _, r := range results {
						o := r.([]interface{})
						records = append(records, makeRecord(fields, o))
					}
					batchResults[responseIndex] = records
				} else {
					errors = append(errors, fmt.Sprintf("Bad response: %s", response.Value))
				}
			} else {
				errors = append(errors, fmt.Sprintf("Query failed with: %s", response.ApplicationProperties["statusDescription"]))
			}
		}
	}
	if len(errors) > 0 {
		return nil, fmt.Errorf(strings.Join(errors, ", "))
	}
	return batchResults, nil
}

func (a *Agent) GetInteriorNodes() ([]RouterNode, error) {
	var address string
	var err error
	if a.isEdgeRouter() {
		address, err = a.getInteriorAddressForUplink()
		if err != nil {
			return nil, fmt.Errorf("Could not determine interior agent address for edge router: %s", err)
		}
	}
	records, err := a.QueryByAgentAddress("org.apache.qpid.dispatch.router.node", []string{}, address)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Interior nodes are %v\n", records)
	nodes := make([]RouterNode, len(records))
	for i, r := range records {
		nodes[i] = asRouterNode(r)
	}
	return nodes, nil
}

func (a *Agent) GetConnections() ([]Connection, error) {
	return a.GetConnectionsFor("")
}

func (a *Agent) GetConnectionsFor(agent string) ([]Connection, error) {
	records, err := a.Query("org.apache.qpid.dispatch.connection", []string{})
	if err != nil {
		return nil, err
	}
	connections := make([]Connection, len(records))
	for i, r := range records {
		connections[i] = asConnection(r)
	}
	return connections, nil
}

func getAddressesFor(routers []Router) []string {
	agents := make([]string, len(routers))
	for i, r := range routers {
		agents[i] = r.Address + "/$management"
	}
	return agents
}

func getBridgeServerAddressesFor(routers []Router) []string {
	agents := make([]string, len(routers))
	for i, r := range routers {
		agents[i] = r.Id + "/bridge-server/$management"
	}
	return agents
}

func (a *Agent) GetAllRouters() ([]Router, error) {
	nodes, err := a.GetInteriorNodes()
	if err != nil {
		return nil, err
	}
	routers := []Router{}
	for _, n := range nodes {
		routers = append(routers, *n.asRouter())
	}
	edges, err := a.getAllEdgeRouters(getAddressesFor(routers))
	if err != nil {
		return nil, err
	}
	routers = append(routers, edges...)
	err = a.getSiteIds(routers)
	if err != nil {
		return nil, err
	}
	err = a.getConnectedTo(routers)
	if err != nil {
		return nil, err
	}
	return routers, nil
}

func (a *Agent) getConnectionsForAll(agents []string) ([]Connection, error) {
	connections := []Connection{}
	results, err := a.BatchQuery(queryAllAgents("org.apache.qpid.dispatch.connection", agents))
	if err != nil {
		return nil, err
	}
	for _, records := range results {
		for _, r := range records {
			connections = append(connections, asConnection(r))
		}
	}
	return connections, nil
}

func (a *Agent) getSiteIds(routers []Router) error {
	results, err := a.BatchQuery(queryAllAgents("org.apache.qpid.dispatch.router", getAddressesFor(routers)))
	if err != nil {
		return err
	}
	for i, records := range results {
		if len(records) == 1 {
			routers[i].SiteId = records[0].AsString("metadata")
		} else {
			return fmt.Errorf("Unexpected number of router records: %d", len(records))
		}
	}
	return nil
}

func (a *Agent) getConnectedTo(routers []Router) error {
	results, err := a.BatchQuery(queryAllAgents("org.apache.qpid.dispatch.connection", getAddressesFor(routers)))
	if err != nil {
		return err
	}
	for i, records := range results {
		routers[i].ConnectedTo = []string{}
		for _, r := range records {
			c := asConnection(r)
			if c.Dir == "out" && (c.Role == "edge" || c.Role == "inter-router") {
				routers[i].ConnectedTo = append(routers[i].ConnectedTo, c.Container)
			}
		}
	}
	return nil
}

func getBridgeTypes() []string {
	return []string{
		"org.apache.qpid.dispatch.router.tcpConnector",
		"org.apache.qpid.dispatch.router.tcpListener",
		"org.apache.qpid.dispatch.router.httpConnector",
		"org.apache.qpid.dispatch.router.httpListener",
		"org.apache.qpid.dispatch.router.http2Connector",
		"org.apache.qpid.dispatch.router.http2Listener",
	}
}

func (a *Agent) GetBridges(routers []Router) ([]Record, error) {
	queries := queryAllAgentsForAllTypes(getBridgeTypes(), getBridgeServerAddressesFor(routers))
	results, err := a.BatchQuery(queries)
	if err != nil {
		return nil, err
	}
	flattened := []Record{}
	for _, records := range results {
		flattened = append(flattened, records...)
	}
	return flattened, nil
}

func (a *Agent) getAllEdgeRouters(agents []string) ([]Router, error) {
	edges := []Router{}

	connections, err := a.getConnectionsForAll(agents)
	if err != nil {
		return nil, err
	}
	for _, c := range connections {
		if c.Role == "edge" && c.Dir == "in" {
			router := Router{
				Id:      c.Container,
				Edge:    true,
				Address: getRouterAddress(c.Container, true),
			}
			edges = append(edges, router)
		}
	}
	return edges, nil
}

func (a *Agent) getEdgeRouters(agent string) ([]Router, error) {
	connections, err := a.GetConnectionsFor(agent)
	if err != nil {
		return nil, err
	}
	edges := []Router{}
	for _, c := range connections {
		if c.Role == "edge" && c.Dir == "in" {
			router := Router{
				Id:      c.Container,
				Edge:    true,
				Address: getRouterAddress(c.Container, true),
			}
			edges = append(edges, router)
		}
	}
	return edges, nil
}

func (a *Agent) getLocalRouter() (*Router, error) {
	records, err := a.Query("org.apache.qpid.dispatch.router", []string{})
	if err != nil {
		return nil, err
	}
	if len(records) == 1 {
		return asRouter(records[0]), nil
	} else {
		return nil, fmt.Errorf("Unexpected number of router records: %d", len(records))
	}
}

func (a *Agent) isEdgeRouter() bool {
	return a.local.Edge
}

func (a *Agent) getInteriorAddressForUplink() (string, error) {
	connections, err := a.GetConnections()
	if err != nil {
		return "", err
	}
	for _, c := range connections {
		if c.Role == "edge" && c.Dir == "out" {
			return getRouterAgentAddress(c.Container, false), nil
		}
	}
	return "", fmt.Errorf("Could not find uplink connection")
}

func (a *Agent) SiteQuery(addresses []string) ([]string, error) {
	fmt.Printf("SiteQuery(%v)\n", addresses)
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	batchResults := make([]string, len(addresses))
	for i, to := range addresses {
		request := amqp.Message{
			Properties: &amqp.MessageProperties{
				To:            to,
				ReplyTo:       a.receiver.Address(),
				CorrelationID: uint64(i),
			},
			Value: nil,
		}
		err := a.anonymous.Send(ctx, &request)
		if err != nil {
			a.Close()
			return nil, fmt.Errorf("Could not send request: %s", err)
		}
	}
	errors := []string{}
	for i := 0; i < len(addresses); i++ {
		fmt.Printf("Waiting for response %d of %d\n", (i + 1), len(addresses))
		response, err := a.receiver.Receive(ctx)
		if err != nil {
			a.Close()
			return nil, fmt.Errorf("Failed to receive reponse: %s", err)
		}
		response.Accept()
		responseIndex, ok := response.Properties.CorrelationID.(uint64)
		if !ok {
			errors = append(errors, fmt.Sprintf("Could not get correct correlation id from response: %#v (%T)", response.Properties.CorrelationID, response.Properties.CorrelationID))
		} else {
			if body, ok := response.Value.(string); ok {
				batchResults[responseIndex] = body
			} else {
				errors = append(errors, fmt.Sprintf("Bad response: %#v", response.Value))
			}
		}
	}
	if len(errors) > 0 {
		return nil, fmt.Errorf(strings.Join(errors, ", "))
	}
	return batchResults, nil
}
