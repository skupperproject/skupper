package service_sync

import (
	jsonencoding "encoding/json"
	"fmt"

	amqp "github.com/interconnectedcloud/go-amqp"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
)

const (
	serviceSyncSubjectV1 string = "service-sync-update"
	serviceSyncSubjectV2 string = "service-sync-update-v2"
)

func encode(in *ServiceUpdate) (*amqp.Message, error) {
	list := []types.ServiceInterface{}

	for _, si := range in.definitions {
		list = append(list, si)
	}

	encoded, err := jsonencoding.Marshal(list)
	if err != nil {
		return nil, err
	}

	var request amqp.Message
	var properties amqp.MessageProperties
	properties.Subject = serviceSyncSubjectV2
	request.Properties = &properties
	request.ApplicationProperties = make(map[string]interface{})
	request.ApplicationProperties["origin"] = in.origin
	request.ApplicationProperties["version"] = in.version

	request.Value = string(encoded)

	return &request, nil
}

func decode(msg *amqp.Message) (ServiceUpdate, error) {
	result := ServiceUpdate{
		definitions: map[string]types.ServiceInterface{},
	}
	subject := msg.Properties.Subject
	if !utils.StringSliceContains([]string{serviceSyncSubjectV2}, subject) {
		return result, fmt.Errorf("Service sync subject not valid: %s", subject)
	}
	origin, ok := msg.ApplicationProperties["origin"].(string)
	if !ok {
		return result, fmt.Errorf("Service sync origin not valid: %v", msg.ApplicationProperties["origin"])
	}
	result.origin = origin

	if version, ok := msg.ApplicationProperties["version"].(string); ok {
		result.version = version
	}

	encoded, ok := msg.Value.(string)
	if !ok {
		return result, fmt.Errorf("Service sync body not valid: %v", msg.Value)
	}

	defs := &types.ServiceInterfaceList{}
	err := defs.ConvertFrom(encoded)
	if err != nil {
		return result, err
	}
	for _, def := range *defs {
		def.Origin = origin
		result.definitions[def.Address] = def
	}

	return result, nil
}
