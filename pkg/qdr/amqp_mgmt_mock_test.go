package qdr

import (
	"flag"
	"reflect"
	"testing"

	"crypto/tls"
	"encoding/json"
	"gotest.tools/assert"
)

var clusterRun = flag.Bool("use-cluster", false, "run tests against a configured cluster")

func TestQDR(t *testing.T) {

	// AsInt -----------------------------------
	goodAsInt := Record{}
	goodAsInt["int8"] = int8(0)
	goodAsInt["int16"] = int16(0)
	goodAsInt["int32"] = int32(0)
	goodAsInt["int64"] = int64(0)

	goodAsInt["uint8"] = uint8(0)
	goodAsInt["uint16"] = uint16(0)
	goodAsInt["uint32"] = uint32(0)
	goodAsInt["uint64"] = uint64(0)

	for key, value := range goodAsInt {
		_, ok := AsInt(value)
		assert.Assert(t, ok, "AsInt failed on type |%s|.", key)
	}

	badAsInt := Record{}
	badAsInt["float32"] = float32(1.0)
	badAsInt["float64"] = float64(1.0)
	badAsInt["string"] = "0"

	for key, value := range badAsInt {
		_, ok := AsInt(value)
		assert.Assert(t, !ok, "AsInt succeeded on type |%s|, but should fail.", key)
	}

	// I guess we can at least test that these next few don't crash...
	badAsInt.AsInt("string")
	asConnection(badAsInt)
	badAsInt.AsUint64("string")

	// AsString ----------------------------------------
	goodAsString := Record{}
	goodAsString["key"] = "value"
	assert.Assert(t, goodAsString.AsString("key") == "value", "AsString failed.")

	notGoodAsString := Record{}
	notGoodAsString["key"] = 0x0BADCAFE
	val := notGoodAsString.AsString("key")
	assert.Assert(t, val == "", "AsString succeeded, returning |%s|, but should have failed.")

	// AsBool ----------------------------------------
	goodAsBool := Record{}
	goodAsBool["key"] = true
	assert.Assert(t, goodAsBool.AsBool("key") == true, "AsBool failed.")

	notGoodAsBool := Record{}
	notGoodAsString["key"] = 0x8BADF00D
	assert.Assert(t, notGoodAsBool.AsBool("key") == false, "AsBool succeeded, but should have failed.")

	// AsRecord ----------------------------------------
	record := Record{}
	record["record"] = make(map[string]interface{})
	recordVal := record.AsRecord("record")
	assert.Assert(t, recordVal != nil, "AsRecord failure.")

	record["not_record"] = 13
	recordVal = record.AsRecord("not_record")
	assert.Assert(t, recordVal == nil, "AsRecord succeeded, but shouldn't.")

	// asRouterNode ------------------------------------
	record["id"] = "my_id"
	record["name"] = "my_name"
	record["address"] = "my_address"
	record["nextHop"] = "my_next_hop"
	router_node := asRouterNode(record)
	if router_node.Id != record["id"] ||
		router_node.Name != record["name"] ||
		router_node.Address != record["address"] ||
		router_node.NextHop != record["nextHop"] {
		assert.Assert(t, router_node.Id == record["id"] && router_node.Name == record["name"] && router_node.Address == record["address"] && router_node.NextHop == record["nextHop"], "asRouterNode failure")
	}

	// asRouter ------------------------------------
	record["metadata"] = "my_metadata"
	router := asRouter(record)
	assert.Assert(t, router.Id == record["id"] && router.Site.Id == record["metadata"] && router.Edge == false, "asRouter failure.")

	record["mode"] = "edge"
	router = asRouter(record)
	assert.Assert(t, router.Edge == true, "asRouter failure. (edge)")

	// RouterNode.asRouter ----------------------------
	router2 := router_node.asRouter()
	assert.Assert(t, router2.Id == router_node.Id && router2.Edge == false, "RouterNode.asRouter failure: Id |%s|  Edge: %t.", router2.Id, router2.Edge)

	// NewAgentPool ------------------------------------
	agentPool := NewAgentPool("my_url", &tls.Config{})
	assert.Assert(t, agentPool.url == "my_url", "NewAgentPool failure")

	// Get ------------------------------------
	agent, err := agentPool.Get()
	assert.Assert(t, agent == nil && err != nil, "AgentPool.Get() should have failed but didn't")

	// isOk ----------------------------------------
	code := 199
	assert.Assert(t, !isOk(code), "isOk failure on code %d", code)
	code = 301
	assert.Assert(t, !isOk(code), "isOk failure on code %d", code)
	code = 250
	assert.Assert(t, isOk(code), "isOk failure on code %d", code)

	// makeRecord -----------------------------------
	rec := makeRecord([]string{"a", "b", "c"}, []interface{}{1, 2, 3})
	assert.Assert(t, rec.AsInt("a") == 1 && rec.AsInt("b") == 2 && rec.AsInt("c") == 3, "makeRecord failure.")

	// stringify -----------------------------------
	items := []interface{}{1, 2, 3, 0xA, 0xB, 0xC}
	strs := stringify(items)
	correct := []string{"1", "2", "3", "10", "11", "12"}
	assert.Assert(t, len(strs) == len(correct), "stringify failure: len %d should be %d", len(strs), len(correct))
	for i, str := range strs {
		assert.Assert(t, str == correct[i], "stringify failure: element %d is |%s|, should be |%s|.", i, str, correct[i])
	}

	// getRouterAddress -----------------------------------
	id := "foo"

	correctAddr := "amqp:/_edge/" + id
	resultAddr := getRouterAddress(id, true)
	assert.Assert(t, correctAddr == resultAddr, "getRouterAddress failure: |%s| should be |%s|", correctAddr, resultAddr)

	correctAddr = "amqp:/_topo/0/" + id
	resultAddr = getRouterAddress(id, false)
	assert.Assert(t, correctAddr == resultAddr, "getRouterAddress failure: |%s| should be |%s|", correctAddr, resultAddr)

	// AsUint64 -----------------------------------
	integers := []interface{}{uint8(12), uint16(12), uint32(12), uint64(12),
		int8(12), int16(12), int32(12), int64(12),
		int(12),
	}
	lyingString := "I am an integer!"

	var val64 uint64
	var ok bool
	for _, i := range integers {
		val64, ok = AsUint64(i)
		assert.Assert(t, ok && val64 == 12, "AsUint64 failed on %T.", i)
	}
	val64, ok = AsUint64(lyingString)
	assert.Assert(t, !ok && val64 == 0, "AsUint64 was fooled by a deceptive string.")

}

func TestSiteMetadata(t *testing.T) {
	a := SiteMetadata{
		Id:      "foo",
		Version: "1.2.3",
	}
	b := getSiteMetadata(getSiteMetadataString(a.Id, a.Version))
	if b.Id != a.Id {
		t.Errorf("Invalid metadata, expected id to be %q got %q", a.Id, b.Id)
	}
	if b.Version != a.Version {
		t.Errorf("Invalid metadata, expected version to be %q got %q", a.Id, b.Id)
	}
	id := "I am not an object"
	c := getSiteMetadata(id)
	if c.Id != id {
		t.Errorf("Invalid metadata, expected id to be %q got %q", id, c.Id)
	}
}

func TestMarshalUnmarshalRecordsWithIntegers(t *testing.T) {

	//Marshaling and un-marshaling a map[string]interface{} with int values changes the format of the numbers to float64
	//https://go.dev/blog/json

	record := Record{}
	record["int"] = 65
	record["number"] = json.Number("66")

	_, ok := AsInt(record["int"])
	assert.Assert(t, ok)

	recordResult := Record{}

	data, err := json.Marshal(record)
	assert.Assert(t, err)

	err = json.Unmarshal(data, &recordResult)
	assert.Assert(t, err)

	assert.Assert(t, reflect.TypeOf(recordResult["int"]).String() == "float64")
	assert.Assert(t, reflect.TypeOf(recordResult["number"]).String() == "float64")

	_, ok = AsInt(recordResult["int"])
	assert.Assert(t, !ok)

	_, ok = AsInt(recordResult["number"])
	assert.Assert(t, !ok)
}
