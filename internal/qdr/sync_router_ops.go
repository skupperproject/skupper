package qdr

import (
	"fmt"
)

//TODO: use this in config-sync

func SyncSslProfilesToRouter(agentPool *AgentPool, desired map[string]SslProfile) error {

	agent, err := agentPool.Get()
	if err != nil {
		return err
	}
	defer agentPool.Put(agent)

	actual, err := agent.GetSslProfiles()
	if err != nil {
		return err
	}

	for _, profile := range desired {
		current, ok := actual[profile.Name]
		if !ok {
			if err := agent.CreateSslProfile(profile); err != nil {
				return err
			}
		}
		if current != profile {
			if err := agent.UpdateSslProfile(profile); err != nil {
				return err
			}
		}
	}
	for _, profile := range actual {
		if _, ok := desired[profile.Name]; !ok {
			if err := agent.Delete("io.skupper.router.sslProfile", profile.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func SyncBridgeConfig(agentPool *AgentPool, desired *BridgeConfig) error {
	agent, err := agentPool.Get()
	if err != nil {
		return fmt.Errorf("Could not get management agent : %s", err)
	}
	var synced bool

	synced, err = syncBridgeConfig(agent, desired)

	agentPool.Put(agent)
	if err != nil {
		return fmt.Errorf("Error while syncing bridge config : %s", err)
	}
	if !synced {
		return fmt.Errorf("Bridge config is not synchronised yet")
	}
	return nil
}

func SyncRouterConfig(agentPool *AgentPool, desired *RouterConfig, checkCertFilesExist bool) error {
	if err := syncConnectors(agentPool, desired, checkCertFilesExist); err != nil {
		return err
	}
	if err := syncListeners(agentPool, desired); err != nil {
		return err
	}
	return nil
}

func syncConnectors(agentPool *AgentPool, desired *RouterConfig, checkCertFilesExist bool) error {
	agent, err := agentPool.Get()
	if err != nil {
		return err
	}
	defer agentPool.Put(agent)

	actual, err := agent.GetLocalConnectors()
	if err != nil {
		return fmt.Errorf("Error retrieving local connectors: %s", err)
	}

	ignorePrefix := "auto-mesh"
	if differences := ConnectorsDifference(actual, desired, &ignorePrefix); !differences.Empty() {
		if err = agent.UpdateConnectorConfig(differences, checkCertFilesExist); err != nil {
			return fmt.Errorf("Error syncing connectors: %s", err)
		}
	}
	return nil
}

func syncListeners(agentPool *AgentPool, desired *RouterConfig) error {
	agent, err := agentPool.Get()
	if err != nil {
		return err
	}
	defer agentPool.Put(agent)

	actual, err := agent.GetLocalListeners()
	if err != nil {
		return fmt.Errorf("Error retrieving local listeners: %s", err)
	}

	if differences := ListenersDifference(FilterListeners(actual, IsNotProtectedListener), desired.GetMatchingListeners(IsNotProtectedListener)); !differences.Empty() {
		if err := agent.UpdateListenerConfig(differences); err != nil {
			return fmt.Errorf("Error syncing listeners: %s", err)
		}
	}
	return nil
}

func syncBridgeConfig(agent *Agent, desired *BridgeConfig) (bool, error) {
	actual, err := agent.GetLocalBridgeConfig()
	if err != nil {
		return false, fmt.Errorf("Error retrieving bridges: %s", err)
	}
	differences := actual.Difference(desired)
	if differences.Empty() {
		return true, nil
	} else {
		if err = agent.UpdateLocalBridgeConfig(differences); err != nil {
			return false, fmt.Errorf("Error syncing bridges: %s", err)
		}
		return false, nil
	}
}
