package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Interface interface{}

func TestSetVercelConfig(t *testing.T) {
	// All of the below env variables code is used to bypass gojsonschema's
	// inability to cast this to a []map[string]interface{}.
	environmentVariables := []ProjectEnvironmentVariable{
		{Key: "TEST_ENVIRONMENT_VARIABLE", Value: "testing", Environment: []string{}},
		{Key: "TEST_ENVIRONMENT_VARIABLE_2", Value: "testing", Environment: []string{"production", "preview"}},
	}

	projectDomains := []ProjectDomain{
		{Domain: "test-domain.com", RedirectStatusCode: 307},
	}

	domains := make([]interface{}, len(projectDomains))
	for i, s := range projectDomains {
		domains[i] = s
	}

	variables := make([]interface{}, len(environmentVariables))
	for i, s := range environmentVariables {
		variables[i] = s
	}

	data := map[string]any{
		"team_id":   "test-team",
		"api_token": "${sops.data.output[\"api_token\"]}",
		"project_config": map[string]any{
			"name":                         "test-project",
			"framework":                    "nextjs",
			"serverless_function_region":   "iad1",
			"build_command":                "next build",
			"root_directory":               "./my-project",
			"manual_production_deployment": true,
			"git_repository": map[string]any{
				"production_branch": "main",
				"type":              "github",
				"repo":              "mach-composer/my-project",
			},
			"environment_variables":            variables,
			"domains":                          domains,
			"protection_bypass_for_automation": true,
			"vercel_authentication": map[string]any{
				"protect_production": true,
			},
			"password_protection": map[string]any{
				"password":           "MyPassword",
				"protect_production": true,
			},
		},
	}

	plugin := NewVercelPlugin()

	err := plugin.SetComponentConfig("my-component", map[string]any{
		"integrations": []string{"vercel"},
	})
	require.NoError(t, err)

	err = plugin.SetSiteConfig("my-site", data)
	require.NoError(t, err)

	result, err := plugin.RenderTerraformResources("my-site")
	require.NoError(t, err)
	assert.Contains(t, result, `api_token = sops.data.output["api_token"]`)

	component, err := plugin.RenderTerraformComponent("my-site", "test-component")
	require.NoError(t, err)
	assert.Contains(t, component.Variables, "name = \"test-project\"")
	assert.Contains(t, component.Variables, "framework = \"nextjs\"")
	assert.Contains(t, component.Variables, "serverless_function_region = \"iad1\"")
	assert.Contains(t, component.Variables, "build_command = \"next build\"")
	assert.Contains(t, component.Variables, "root_directory = \"./my-project\"")
	assert.Contains(t, component.Variables, "vercel_team_id = \"test-team\"")
	assert.Contains(t, component.Variables, "manual_production_deployment = true")
	assert.Contains(t, component.Variables, "production_branch = \"main\"")
	assert.Contains(t, component.Variables, "type = \"github\"")
	assert.Contains(t, component.Variables, "repo = \"mach-composer/my-project\"")
	assert.Contains(t, component.Variables, "protection_bypass_for_automation = true")
	assert.Contains(t, component.Variables, "protect_production = true")
	assert.Contains(t, component.Variables, "password = \"MyPassword\"")

	// Test environment variables

	// Test default response
	assert.Contains(t, component.Variables, "environment = [\"development\", \"preview\", \"production\"]")
	// Test custom environment variables list
	assert.Contains(t, component.Variables, "environment = [\"production\", \"preview\"]")

	// Test domains
	assert.Contains(t, component.Variables, "domain = \"test-domain.com\"")
	assert.Contains(t, component.Variables, "redirect_status_code = 307")

}

func TestInheritance(t *testing.T) {
	globalEnvironmentVariables := []ProjectEnvironmentVariable{
		{Key: "TEST_ENVIRONMENT_VARIABLE", Value: "testing", Environment: []string{}},
		{Key: "TEST_EXTEND_VARIABLE", Value: "test", Environment: []string{"production"}},
	}
	globalVariables := make([]interface{}, len(globalEnvironmentVariables))
	for i, s := range globalEnvironmentVariables {
		globalVariables[i] = s
	}
	globalData := map[string]any{
		"team_id":   "test-team",
		"api_token": "test-token",
		"project_config": map[string]any{
			"manual_production_deployment":     true,
			"protection_bypass_for_automation": true,
			"vercel_authentication": map[string]any{
				"protect_production": true,
			},
			"password_protection": map[string]any{
				"password":           "MyPassword",
				"protect_production": true,
			},
			"environment_variables": globalVariables,
		},
	}

	siteEnvironmentVariables := []ProjectEnvironmentVariable{
		{Key: "TEST_ENVIRONMENT_VARIABLE_2", Value: "testing", Environment: []string{"production", "preview"}},
		{Key: "TEST_EXTEND_VARIABLE", Value: "testing", Environment: []string{"production", "preview", "development"}},
	}
	siteVariables := make([]interface{}, len(siteEnvironmentVariables))
	for i, s := range siteEnvironmentVariables {
		siteVariables[i] = s
	}

	siteData := map[string]any{
		"team_id":   "test-team-override",
		"api_token": "test-token-override",
		"project_config": map[string]any{
			"vercel_authentication": map[string]any{
				"protect_production": false,
			},
			"password_protection": map[string]any{
				"protect_production": false,
			},
			"environment_variables": siteVariables,
		},
	}

	plugin := NewVercelPlugin()

	err := plugin.SetGlobalConfig(globalData)
	require.NoError(t, err)

	err = plugin.SetSiteConfig("my-site", siteData)
	require.NoError(t, err)

	result, err := plugin.RenderTerraformResources("my-site")
	require.NoError(t, err)
	assert.Contains(t, result, "api_token = \"test-token-override\"")

	component, err := plugin.RenderTerraformComponent("my-site", "test-component")
	require.NoError(t, err)

	// Test overriding fields
	assert.Contains(t, component.Variables, "vercel_team_id = \"test-team-override\"")

	// Test whether environment variables get extended
	assert.Contains(t, component.Variables, "environment = [\"development\", \"preview\", \"production\"]")
	assert.Contains(t, component.Variables, "environment = [\"production\", \"preview\"]")
	assert.Contains(t, component.Variables, "protect_production = false")

	assert.Contains(t, component.Variables, "environment")
}

func TestSiteComponentInheritance(t *testing.T) {
	siteEnvironmentVariables := []ProjectEnvironmentVariable{
		{Key: "TEST_ENVIRONMENT_VARIABLE_2", Value: "testing", Environment: []string{"production", "preview"}},
		{Key: "TEST_EXTEND_VARIABLE", Value: "testing", Environment: []string{"production", "preview", "development"}},
	}
	siteVariables := make([]interface{}, len(siteEnvironmentVariables))
	for i, s := range siteEnvironmentVariables {
		siteVariables[i] = s
	}

	siteData := map[string]any{
		"team_id":   "test-team-override",
		"api_token": "test-token-override",
		"project_config": map[string]any{
			"serverless_function_region":   "iad1",
			"manual_production_deployment": false,
			"environment_variables":        siteVariables,
		},
	}

	componentEnvironmentVariables := []ProjectEnvironmentVariable{
		{Key: "TEST_ENVIRONMENT_VARIABLE_3", Value: "testing"},
	}

	componentVariables := make([]interface{}, len(componentEnvironmentVariables))
	for i, s := range componentEnvironmentVariables {
		componentVariables[i] = s
	}

	componentData := map[string]any{
		"project_config": map[string]any{
			"serverless_function_region":   "fra1",
			"manual_production_deployment": true,
			"environment_variables":        componentVariables,
		},
	}

	plugin := NewVercelPlugin()

	err := plugin.SetSiteConfig("my-site", siteData)
	require.NoError(t, err)

	err = plugin.SetSiteComponentConfig("my-site", "test-component", componentData)

	component, err := plugin.RenderTerraformComponent("my-site", "test-component")
	require.NoError(t, err)

	// Test whether environment variables get extended
	assert.Contains(t, component.Variables, "vercel_project_serverless_function_region = \"fra1\"")
	assert.Contains(t, component.Variables, "vercel_project_manual_production_deployment = true")
	assert.Contains(t, component.Variables, "key = \"TEST_ENVIRONMENT_VARIABLE_2\"")
	assert.Contains(t, component.Variables, "key = \"TEST_ENVIRONMENT_VARIABLE_3\"")

}

func TestExtendEnvironmentVariables(t *testing.T) {
	globalEnvironmentVariables := []ProjectEnvironmentVariable{
		{Key: "TEST_EXTEND_VARIABLE", Value: "test", Environment: []string{"production"}},
	}
	globalVariables := make([]interface{}, len(globalEnvironmentVariables))
	for i, s := range globalEnvironmentVariables {
		globalVariables[i] = s
	}
	globalData := map[string]any{
		"team_id":   "test-team",
		"api_token": "test-token",
		"project_config": map[string]any{
			"manual_production_deployment": true,
			"environment_variables":        globalVariables,
		},
	}

	siteEnvironmentVariables := []ProjectEnvironmentVariable{
		{Key: "TEST_EXTEND_VARIABLE", Value: "testing", Environment: []string{"production", "preview", "development"}},
	}
	siteVariables := make([]interface{}, len(siteEnvironmentVariables))
	for i, s := range siteEnvironmentVariables {
		siteVariables[i] = s
	}

	siteData := map[string]any{
		"team_id":   "test-team",
		"api_token": "test-token",
		"project_config": map[string]any{
			"manual_production_deployment": true,
			"environment_variables":        siteVariables,
		},
	}

	plugin := NewVercelPlugin()

	err := plugin.SetGlobalConfig(globalData)
	require.NoError(t, err)

	err = plugin.SetSiteConfig("my-site", siteData)
	require.NoError(t, err)

	// Test whether environment variables get extended
	component, err := plugin.RenderTerraformComponent("my-site", "test-component")
	require.NoError(t, err)

	// Should only contain the site extended variable content
	assert.Contains(t, component.Variables, "environment = [\"production\", \"preview\", \"development\"]")
	assert.Contains(t, component.Variables, "value = \"testing\"")

}

func TestCompleteInheritance(t *testing.T) {
	global := map[string]any{
		"team_id": "test-team",
		"project_config": map[string]any{
			"serverless_function_region": "fra1",
		},
	}

	plugin := NewVercelPlugin()

	err := plugin.SetGlobalConfig(global)
	require.NoError(t, err)

	siteConfig := map[string]any{
		"project_config": map[string]any{
			"git_repository": map[string]any{
				"production_branch": "production",
				"type":              "github",
				"repo":              "owner/test-repo",
			},
		},
	}

	err = plugin.SetSiteConfig("my-site", siteConfig)
	require.NoError(t, err)

	componentConfig := map[string]any{
		"project_config": map[string]any{
			"manual_production_deployment": true,
		},
	}

	err = plugin.SetSiteComponentConfig("my-site", "test-component", componentConfig)
	require.NoError(t, err)

	component, err := plugin.RenderTerraformComponent("my-site", "test-component")

	assert.Contains(t, component.Variables, "vercel_project_serverless_function_region = \"fra1\"")
	assert.Contains(t, component.Variables, "type = \"github\"")
	assert.Contains(t, component.Variables, "production_branch = \"production\"")
	assert.Contains(t, component.Variables, "vercel_project_manual_production_deployment = true")
}
