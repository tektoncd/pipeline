package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mitchellh/mapstructure"
)

// ListPluginsInput is used as input to the ListPlugins function.
type ListPluginsInput struct {
	// Type of the plugin. Required.
	Type PluginType `json:"type"`
}

// ListPluginsResponse is the response from the ListPlugins call.
type ListPluginsResponse struct {
	// PluginsByType is the list of plugins by type.
	PluginsByType map[PluginType][]string `json:"types"`

	Details []PluginDetails `json:"details,omitempty"`

	// Names is the list of names of the plugins.
	//
	// Deprecated: Newer server responses should be returning PluginsByType (json:
	// "types") instead.
	Names []string `json:"names"`
}

type PluginDetails struct {
	Type              string `json:"type"`
	Name              string `json:"name"`
	Version           string `json:"version,omitempty"`
	Builtin           bool   `json:"builtin"`
	DeprecationStatus string `json:"deprecation_status,omitempty" mapstructure:"deprecation_status"`
}

// ListPlugins wraps ListPluginsWithContext using context.Background.
func (c *Sys) ListPlugins(i *ListPluginsInput) (*ListPluginsResponse, error) {
	return c.ListPluginsWithContext(context.Background(), i)
}

// ListPluginsWithContext lists all plugins in the catalog and returns their names as a
// list of strings.
func (c *Sys) ListPluginsWithContext(ctx context.Context, i *ListPluginsInput) (*ListPluginsResponse, error) {
	ctx, cancelFunc := c.c.withConfiguredTimeout(ctx)
	defer cancelFunc()

	resp, err := c.c.rawRequestWithContext(ctx, c.c.NewRequest(http.MethodGet, "/v1/sys/plugins/catalog"))
	if err != nil && resp == nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	defer resp.Body.Close()

	secret, err := ParseSecret(resp.Body)
	if err != nil {
		return nil, err
	}
	if secret == nil || secret.Data == nil {
		return nil, errors.New("data from server response is empty")
	}

	result := &ListPluginsResponse{
		PluginsByType: make(map[PluginType][]string),
	}
	switch i.Type {
	case PluginTypeUnknown:
		for _, pluginType := range PluginTypes {
			pluginsRaw, ok := secret.Data[pluginType.String()]
			if !ok {
				continue
			}

			pluginsIfc, ok := pluginsRaw.([]interface{})
			if !ok {
				return nil, fmt.Errorf("unable to parse plugins for %q type", pluginType.String())
			}

			plugins := make([]string, 0, len(pluginsIfc))
			for _, nameIfc := range pluginsIfc {
				name, ok := nameIfc.(string)
				if !ok {
					continue
				}
				plugins = append(plugins, name)
			}
			result.PluginsByType[pluginType] = plugins
		}
	default:
		pluginsRaw, ok := secret.Data[i.Type.String()]
		if !ok {
			return nil, fmt.Errorf("no %s entry in returned data", i.Type.String())
		}

		var respKeys []string
		if err := mapstructure.Decode(pluginsRaw, &respKeys); err != nil {
			return nil, err
		}
		result.PluginsByType[i.Type] = respKeys
	}

	if detailed, ok := secret.Data["detailed"]; ok {
		var details []PluginDetails
		if err := mapstructure.Decode(detailed, &details); err != nil {
			return nil, err
		}

		switch i.Type {
		case PluginTypeUnknown:
			result.Details = details
		default:
			// Filter for just the queried type.
			for _, entry := range details {
				if entry.Type == i.Type.String() {
					result.Details = append(result.Details, entry)
				}
			}
		}
	}

	return result, nil
}

// GetPluginInput is used as input to the GetPlugin function.
type GetPluginInput struct {
	Name string `json:"-"`

	// Type of the plugin. Required.
	Type    PluginType `json:"type"`
	Version string     `json:"version"`
}

// GetPluginResponse is the response from the GetPlugin call.
type GetPluginResponse struct {
	Args              []string `json:"args"`
	Builtin           bool     `json:"builtin"`
	Command           string   `json:"command"`
	Name              string   `json:"name"`
	SHA256            string   `json:"sha256"`
	DeprecationStatus string   `json:"deprecation_status,omitempty"`
	Version           string   `json:"version,omitempty"`
}

// GetPlugin wraps GetPluginWithContext using context.Background.
func (c *Sys) GetPlugin(i *GetPluginInput) (*GetPluginResponse, error) {
	return c.GetPluginWithContext(context.Background(), i)
}

// GetPluginWithContext retrieves information about the plugin.
func (c *Sys) GetPluginWithContext(ctx context.Context, i *GetPluginInput) (*GetPluginResponse, error) {
	ctx, cancelFunc := c.c.withConfiguredTimeout(ctx)
	defer cancelFunc()

	path := catalogPathByType(i.Type, i.Name)
	req := c.c.NewRequest(http.MethodGet, path)
	if i.Version != "" {
		req.Params.Set("version", i.Version)
	}

	resp, err := c.c.rawRequestWithContext(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data *GetPluginResponse
	}
	err = resp.DecodeJSON(&result)
	if err != nil {
		return nil, err
	}
	return result.Data, err
}

// RegisterPluginInput is used as input to the RegisterPlugin function.
type RegisterPluginInput struct {
	// Name is the name of the plugin. Required.
	Name string `json:"-"`

	// Type of the plugin. Required.
	Type PluginType `json:"type"`

	// Args is the list of args to spawn the process with.
	Args []string `json:"args,omitempty"`

	// Command is the command to run.
	Command string `json:"command,omitempty"`

	// SHA256 is the shasum of the plugin.
	SHA256 string `json:"sha256,omitempty"`

	// Version is the optional version of the plugin being registered
	Version string `json:"version,omitempty"`
}

// RegisterPlugin wraps RegisterPluginWithContext using context.Background.
func (c *Sys) RegisterPlugin(i *RegisterPluginInput) error {
	return c.RegisterPluginWithContext(context.Background(), i)
}

// RegisterPluginWithContext registers the plugin with the given information.
func (c *Sys) RegisterPluginWithContext(ctx context.Context, i *RegisterPluginInput) error {
	ctx, cancelFunc := c.c.withConfiguredTimeout(ctx)
	defer cancelFunc()

	path := catalogPathByType(i.Type, i.Name)
	req := c.c.NewRequest(http.MethodPut, path)

	if err := req.SetJSONBody(i); err != nil {
		return err
	}

	resp, err := c.c.rawRequestWithContext(ctx, req)
	if err == nil {
		defer resp.Body.Close()
	}
	return err
}

// DeregisterPluginInput is used as input to the DeregisterPlugin function.
type DeregisterPluginInput struct {
	// Name is the name of the plugin. Required.
	Name string `json:"-"`

	// Type of the plugin. Required.
	Type PluginType `json:"type"`

	// Version of the plugin. Optional.
	Version string `json:"version,omitempty"`
}

// DeregisterPlugin wraps DeregisterPluginWithContext using context.Background.
func (c *Sys) DeregisterPlugin(i *DeregisterPluginInput) error {
	return c.DeregisterPluginWithContext(context.Background(), i)
}

// DeregisterPluginWithContext removes the plugin with the given name from the plugin
// catalog.
func (c *Sys) DeregisterPluginWithContext(ctx context.Context, i *DeregisterPluginInput) error {
	ctx, cancelFunc := c.c.withConfiguredTimeout(ctx)
	defer cancelFunc()

	path := catalogPathByType(i.Type, i.Name)
	req := c.c.NewRequest(http.MethodDelete, path)
	req.Params.Set("version", i.Version)
	resp, err := c.c.rawRequestWithContext(ctx, req)
	if err == nil {
		defer resp.Body.Close()
	}
	return err
}

// ReloadPluginInput is used as input to the ReloadPlugin function.
type ReloadPluginInput struct {
	// Plugin is the name of the plugin to reload, as registered in the plugin catalog
	Plugin string `json:"plugin"`

	// Mounts is the array of string mount paths of the plugin backends to reload
	Mounts []string `json:"mounts"`

	// Scope is the scope of the plugin reload
	Scope string `json:"scope"`
}

// ReloadPlugin wraps ReloadPluginWithContext using context.Background.
func (c *Sys) ReloadPlugin(i *ReloadPluginInput) (string, error) {
	return c.ReloadPluginWithContext(context.Background(), i)
}

// ReloadPluginWithContext reloads mounted plugin backends, possibly returning
// reloadId for a cluster scoped reload
func (c *Sys) ReloadPluginWithContext(ctx context.Context, i *ReloadPluginInput) (string, error) {
	ctx, cancelFunc := c.c.withConfiguredTimeout(ctx)
	defer cancelFunc()

	path := "/v1/sys/plugins/reload/backend"
	req := c.c.NewRequest(http.MethodPut, path)

	if err := req.SetJSONBody(i); err != nil {
		return "", err
	}

	resp, err := c.c.rawRequestWithContext(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if i.Scope == "global" {
		// Get the reload id
		secret, parseErr := ParseSecret(resp.Body)
		if parseErr != nil {
			return "", parseErr
		}
		if _, ok := secret.Data["reload_id"]; ok {
			return secret.Data["reload_id"].(string), nil
		}
	}
	return "", err
}

// ReloadStatus is the status of an individual node's plugin reload
type ReloadStatus struct {
	Timestamp time.Time `json:"timestamp" mapstructure:"timestamp"`
	Error     string    `json:"error" mapstructure:"error"`
}

// ReloadStatusResponse is the combined response of all known completed plugin reloads
type ReloadStatusResponse struct {
	ReloadID string                   `mapstructure:"reload_id"`
	Results  map[string]*ReloadStatus `mapstructure:"results"`
}

// ReloadPluginStatusInput is used as input to the ReloadStatusPlugin function.
type ReloadPluginStatusInput struct {
	// ReloadID is the ID of the reload operation
	ReloadID string `json:"reload_id"`
}

// ReloadPluginStatus wraps ReloadPluginStatusWithContext using context.Background.
func (c *Sys) ReloadPluginStatus(reloadStatusInput *ReloadPluginStatusInput) (*ReloadStatusResponse, error) {
	return c.ReloadPluginStatusWithContext(context.Background(), reloadStatusInput)
}

// ReloadPluginStatusWithContext retrieves the status of a reload operation
func (c *Sys) ReloadPluginStatusWithContext(ctx context.Context, reloadStatusInput *ReloadPluginStatusInput) (*ReloadStatusResponse, error) {
	ctx, cancelFunc := c.c.withConfiguredTimeout(ctx)
	defer cancelFunc()

	path := "/v1/sys/plugins/reload/backend/status"
	req := c.c.NewRequest(http.MethodGet, path)
	req.Params.Add("reload_id", reloadStatusInput.ReloadID)

	resp, err := c.c.rawRequestWithContext(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp != nil {
		secret, parseErr := ParseSecret(resp.Body)
		if parseErr != nil {
			return nil, err
		}

		var r ReloadStatusResponse
		d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeHookFunc(time.RFC3339),
			Result:     &r,
		})
		if err != nil {
			return nil, err
		}
		err = d.Decode(secret.Data)
		if err != nil {
			return nil, err
		}
		return &r, nil
	}
	return nil, nil
}

// catalogPathByType is a helper to construct the proper API path by plugin type
func catalogPathByType(pluginType PluginType, name string) string {
	path := fmt.Sprintf("/v1/sys/plugins/catalog/%s/%s", pluginType, name)

	// Backwards compat, if type is not provided then use old path
	if pluginType == PluginTypeUnknown {
		path = fmt.Sprintf("/v1/sys/plugins/catalog/%s", name)
	}

	return path
}
