package agent

import (
	"context"
	"go.uber.org/zap"
	"hash/fnv"
	"in-cluster/pkg/kube_api"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
)

const localConfig = `
exporters:
  otlp:
    endpoint: localhost:1111

receivers:
  otlp:
    protocols:
      grpc: {}
      http: {}

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: []
      exporters: [otlp]
`

type Agent struct {
	logger *zap.SugaredLogger

	agentType    string
	agentVersion string

	instanceId ulid.ULID

	agentDescription *protobufs.AgentDescription

	opampClient client.OpAMPClient

	remoteConfigStatus *protobufs.RemoteConfigStatus

	k8sAPIClient kube_api.K8sAPIClient

	//hash to stop infinite loop
	hash map[uint64]struct{}
}

func NewAgent(logger *zap.SugaredLogger, agentType string, agentVersion string) *Agent {
	agent := &Agent{
		logger:       logger,
		agentType:    agentType,
		agentVersion: agentVersion,
		k8sAPIClient: kube_api.NewClient(logger),
		hash:         make(map[uint64]struct{}),
	}

	agent.createAgentIdentity()
	agent.logger.Debugf("Agent starting, id=%v, type=%s, version=%s.",
		agent.instanceId.String(), agentType, agentVersion)

	//agent.loadLocalConfig()
	if err := agent.start(); err != nil {
		agent.logger.Errorf("Cannot start OpAMP client: %v", err)
		return nil
	}

	return agent
}

func (agent *Agent) start() error {
	agent.opampClient = client.NewHTTP(agent.logger)

	settings := types.StartSettings{
		OpAMPServerURL: "http://host.minikube.internal:3000/v1/opamp",
		InstanceUid:    agent.instanceId.String(),
		Callbacks: types.CallbacksStruct{
			OnConnectFunc: func() {
				agent.logger.Debugf("Connected to the server.")
			},
			OnConnectFailedFunc: func(err error) {
				agent.logger.Errorf("Failed to connect to the server: %v", err)
			},
			OnErrorFunc: func(err *protobufs.ServerErrorResponse) {
				agent.logger.Errorf("Server returned an error response: %v", err.ErrorMessage)
			},
			SaveRemoteConfigStatusFunc: func(_ context.Context, status *protobufs.RemoteConfigStatus) {
				agent.remoteConfigStatus = status
			},
			GetEffectiveConfigFunc: func(ctx context.Context) (*protobufs.EffectiveConfig, error) {
				return agent.composeEffectiveConfig(), nil
			},
			OnMessageFunc: agent.onMessage,
		},
		RemoteConfigStatus: agent.remoteConfigStatus,
	}
	err := agent.opampClient.SetAgentDescription(agent.agentDescription)
	if err != nil {
		return err
	}

	agent.logger.Debugf("Starting OpAMP client...")

	err = agent.opampClient.Start(context.Background(), settings)
	if err != nil {
		return err
	}

	agent.logger.Debugf("OpAMP Client started.")

	return nil
}

func (agent *Agent) createAgentIdentity() {
	// Generate instance id.
	entropy := ulid.Monotonic(rand.New(rand.NewSource(0)), 0)
	agent.instanceId = ulid.MustNew(ulid.Timestamp(time.Now()), entropy)

	hostname, _ := os.Hostname()

	// Create Agent description.
	agent.agentDescription = &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "service.name",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{StringValue: agent.agentType},
				},
			},
			{
				Key: "service.version",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{StringValue: agent.agentVersion},
				},
			},
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "os.family",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: runtime.GOOS,
					},
				},
			},
			{
				Key: "host.name",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: hostname,
					},
				},
			},
		},
	}
}

func (agent *Agent) updateAgentIdentity(instanceId ulid.ULID) {
	agent.logger.Debugf("Agent identify is being changed from id=%v to id=%v",
		agent.instanceId.String(),
		instanceId.String())
	agent.instanceId = instanceId

}

/*
func (agent *Agent) loadLocalConfig() {
	var k = koanf.New(".")
	_ = k.Load(rawbytes.Provider([]byte(agent.effectiveConfig)), yaml.Parser())

	effectiveConfigBytes, err := k.Marshal(yaml.Parser())
	if err != nil {
		panic(err)
	}

	agent.effectiveConfig = string(effectiveConfigBytes)
}
*/
func (agent *Agent) composeEffectiveConfig() *protobufs.EffectiveConfig {
	return &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {Body: []byte{}},
			},
		},
	}
}

type agentConfigFileItem struct {
	name string
	file *protobufs.AgentConfigFile
}

type agentConfigFileSlice []agentConfigFileItem

func (a agentConfigFileSlice) Less(i, j int) bool {
	return a[i].name < a[j].name
}

func (a agentConfigFileSlice) Swap(i, j int) {
	t := a[i]
	a[i] = a[j]
	a[j] = t
}

func (a agentConfigFileSlice) Len() int {
	return len(a)
}

/*
func (agent *Agent) applyRemoteConfig(config *protobufs.AgentRemoteConfig) (configChanged bool, err error) {
	if config == nil {
		return false, nil
	}

	agent.logger.Debugf("Received remote config from server, hash=%x.", config.ConfigHash)

	// Begin with local config. We will later merge received configs on top of it.
	var k = koanf.New(".")
	if err := k.Load(rawbytes.Provider([]byte(agent.effectiveConfig)), yaml.Parser()); err != nil {
		return false, err
	}

	orderedConfigs := agentConfigFileSlice{}
	for name, file := range config.Config.ConfigMap {
		if name == "" {
			// skip instance config
			continue
		}
		orderedConfigs = append(orderedConfigs, agentConfigFileItem{
			name: name,
			file: file,
		})
	}

	// Sort to make sure the order of merging is stable.
	sort.Sort(orderedConfigs)

	// Append instance config as the last item.
	instanceConfig := config.Config.ConfigMap[""]
	if instanceConfig != nil {
		orderedConfigs = append(orderedConfigs, agentConfigFileItem{
			name: "",
			file: instanceConfig,
		})
	}

	// Merge received configs.
	for _, item := range orderedConfigs {
		var k2 = koanf.New(".")
		err := k2.Load(rawbytes.Provider(item.file.Body), yaml.Parser())
		if err != nil {
			return false, fmt.Errorf("cannot parse config named %s: %v", item.name, err)
		}
		err = k.Merge(k2)
		if err != nil {
			return false, fmt.Errorf("cannot merge config named %s: %v", item.name, err)
		}
	}

	// The merged final result is our effective config.
	effectiveConfigBytes, err := k.Marshal(yaml.Parser())
	if err != nil {
		panic(err)
	}

	newEffectiveConfig := string(effectiveConfigBytes)
	configChanged = false
	if agent.effectiveConfig != newEffectiveConfig {
		agent.logger.Debugf("Effective config changed. Need to report to server.")
		agent.effectiveConfig = newEffectiveConfig
		configChanged = true
	}

	return configChanged, nil
}
*/
func (agent *Agent) Shutdown() {
	agent.logger.Debugf("Agent shutting down...")
	if agent.opampClient != nil {
		_ = agent.opampClient.Stop(context.Background())
	}
}

/*
func (agent *Agent) onMessage(ctx context.Context, msg *types.MessageData) {
	configChanged := false
	if msg.RemoteConfig != nil {
		var err error
		configChanged, err = agent.applyRemoteConfig(msg.RemoteConfig)
		agent.logger.Debugf("Config has changed: %v", configChanged)
		if err != nil {
			agent.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
				LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
				Status:               protobufs.RemoteConfigStatus_FAILED,
				ErrorMessage:         err.Error(),
			})
		} else {
			if configChanged {
				if err = agent.k8sAPIClient.Orchestrate(agent.effectiveConfig); err != nil {
					agent.logger.Errorf("Error: %w", err)
					agent.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
						LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
						Status:               protobufs.RemoteConfigStatus_FAILED,
						ErrorMessage:         err.Error(),
					})
				}
			}
			agent.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
				LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
				Status:               protobufs.RemoteConfigStatus_APPLIED,
			})
		}
	}

	if msg.AgentIdentification != nil {
		newInstanceId, err := ulid.Parse(msg.AgentIdentification.NewInstanceUid)
		if err != nil {
			agent.logger.Errorf(err.Error())
		}
		agent.updateAgentIdentity(newInstanceId)
	}

	if configChanged {
		err := agent.opampClient.UpdateEffectiveConfig(ctx)
		if err != nil {
			agent.logger.Errorf(err.Error())
		}
	}
}
*/
func (agent *Agent) onMessage(ctx context.Context, msg *types.MessageData) {
	var configChanged bool
	if msg.RemoteConfig != nil {
		var err error
		for _, v := range msg.RemoteConfig.Config.ConfigMap {
			agent.logger.Debugf("Received config: %s", string(v.Body))
			hash := generateHash(v.Body)
			if _, ok := agent.hash[hash]; !ok {
				if err = agent.k8sAPIClient.Orchestrate(v.Body, v.ContentType); err != nil {
					agent.logger.Errorf("Error: %w", err)
					agent.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
						LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
						Status:               protobufs.RemoteConfigStatus_FAILED,
						ErrorMessage:         err.Error(),
					})
				}
				agent.hash[hash] = struct{}{}
				configChanged = true
			} else {
				agent.logger.Debugf("config provided is same as already applied, hence ignoring it")
			}
		}
		agent.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
			LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
			Status:               protobufs.RemoteConfigStatus_APPLIED,
		})

		if msg.AgentIdentification != nil {
			newInstanceId, err := ulid.Parse(msg.AgentIdentification.NewInstanceUid)
			if err != nil {
				agent.logger.Errorf(err.Error())
			}
			agent.updateAgentIdentity(newInstanceId)
		}
		if configChanged {
			if err = agent.opampClient.UpdateEffectiveConfig(ctx); err != nil {
				agent.logger.Errorf(err.Error())
			}
		}
	}
}

func generateHash(content []byte) uint64 {
	h := fnv.New64()
	h.Write(content)
	return h.Sum64()
}
