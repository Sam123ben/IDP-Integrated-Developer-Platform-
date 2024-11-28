// backend/handlers/generate.go

package handlers

import (
	"backend/models"
	"backend/utils"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
)

// GenerateTerraformHandler handles HTTP requests to generate Terraform files.
func GenerateTerraformHandler(w http.ResponseWriter, r *http.Request) {
	var req models.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := GenerateTerraform(&req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Terraform code generated successfully"))
}

// GenerateTerraform processes the request to generate Terraform files.
func GenerateTerraform(req *models.GenerateRequest) error {
	if req.OrganisationName == "" || req.ProductName == "" || req.Provider == "" {
		return errors.New("organisation_name, product_name, and provider are required")
	}

	// Load configuration from terraform-generator.json
	config, err := utils.LoadConfig("configs/terraform-generator.json")
	if err != nil {
		return err
	}

	// Filter provider data based on the input provider
	providerData := filterProviderData(config.Providers, req.Provider)
	if providerData == nil {
		return errors.New("specified provider not found in configuration")
	}

	basePath := filepath.Join("output", req.Provider, req.OrganisationName)

	if len(req.Customers) > 0 {
		return processCustomers(req, config, basePath, providerData)
	}

	// Process for a single product
	productPath := filepath.Join(basePath, req.ProductName)
	if err := utils.CreateDirectories([]string{filepath.Join(productPath, "backend")}); err != nil {
		return err
	}

	return generateProductFiles(req, config, productPath, providerData)
}

// filterProviderData filters provider details based on the specified provider name.
func filterProviderData(providers []models.Provider, providerName string) *models.Provider {
	aliases := map[string]string{
		"azure":   "azurerm",
		"aws":     "aws",
		"gcp":     "google",
		"azurerm": "azurerm", // Keep the original name as well
		"google":  "google",
	}

	normalizedProvider := aliases[strings.ToLower(providerName)]

	for _, provider := range providers {
		if strings.EqualFold(provider.Name, normalizedProvider) {
			return &provider
		}
	}
	return nil
}

// processCustomers generates Terraform files for multiple customers.
func processCustomers(req *models.GenerateRequest, config *models.Config, basePath string, provider *models.Provider) error {
	for _, customer := range req.Customers {
		customer = strings.TrimSpace(customer)
		customerPath := filepath.Join(basePath, customer)
		paths := []string{
			filepath.Join(customerPath, "backend"),
			filepath.Join(customerPath, "vars"),
		}

		// Create directories
		if err := utils.CreateDirectories(paths); err != nil {
			return err
		}

		// Generate files for the customer
		if err := generateCustomerFiles(req, config, customerPath, customer, provider); err != nil {
			return err
		}
	}
	return nil
}

// generateProductFiles creates Terraform files for a single product.
func generateProductFiles(req *models.GenerateRequest, config *models.Config, productPath string, provider *models.Provider) error {
	data := prepareTemplateData(req, config, provider, "")

	// Generate files
	if err := generateTerraformFiles(productPath, data, req.Provider, req.ProductName); err != nil {
		return err
	}

	// Generate backend tfvars files
	return generateBackendTfvarsFiles(productPath, data, req.ProductName)
}

// generateCustomerFiles creates Terraform files for a single customer.
func generateCustomerFiles(req *models.GenerateRequest, config *models.Config, customerPath, customerName string, provider *models.Provider) error {
	data := prepareTemplateData(req, config, provider, customerName)

	// Generate files
	if err := generateTerraformFiles(customerPath, data, req.Provider, customerName); err != nil {
		return err
	}

	// Generate backend and vars tfvars files
	return generateBackendAndVarsTfvarsFiles(customerPath, data, customerName)
}

// prepareTemplateData prepares data for the templates.
func prepareTemplateData(req *models.GenerateRequest, config *models.Config, provider *models.Provider, customerName string) map[string]interface{} {
	return map[string]interface{}{
		"Provider":         provider,
		"TerraformVersion": config.TerraformVersion,
		"Modules":          config.Modules,
		"OrganisationName": req.OrganisationName,
		"ProductName":      req.ProductName,
		"CustomerName":     customerName,
		"Region":           config.Region,
		"Environment":      config.Environment,
		"Backend":          config.Backend,
		"Variables":        config.Variables,
	}
}

// generateTerraformFiles creates Terraform files like providers.tf, main.tf, variables.tf, and vars.tfvars.
func generateTerraformFiles(path string, data map[string]interface{}, provider, entityName string) error {
	files := []struct {
		Template string
		Dest     string
	}{
		{Template: filepath.Join("templates", "generic", "providers.tf.tmpl"), Dest: filepath.Join(path, "providers.tf")},
		{Template: filepath.Join("templates", provider, "main.tf.tmpl"), Dest: filepath.Join(path, "main.tf")},
		{Template: filepath.Join("templates", "generic", "variables.tf.tmpl"), Dest: filepath.Join(path, "variables.tf")},
		{Template: filepath.Join("templates", "generic", "vars.tfvars.tmpl"), Dest: filepath.Join(path, "vars.tfvars")}, // Added vars.tfvars generation
	}

	for _, file := range files {
		if err := utils.GenerateFileFromTemplate(file.Template, file.Dest, data); err != nil {
			return err
		}
	}
	return nil
}

// generateBackendTfvarsFiles creates backend tfvars files for a product.
func generateBackendTfvarsFiles(path string, data map[string]interface{}, productName string) error {
	environments := []string{"nonprod", "prod"}
	for _, env := range environments {
		data["Environment"] = env
		filename := productName + "_" + env + ".tfvars"
		destPath := filepath.Join(path, "backend", filename)
		if err := utils.GenerateFileFromTemplate(filepath.Join("templates", "generic", "backend.tfvars.tmpl"), destPath, data); err != nil {
			return err
		}
	}
	return nil
}

// generateBackendAndVarsTfvarsFiles creates backend and vars tfvars files for a customer.
func generateBackendAndVarsTfvarsFiles(path string, data map[string]interface{}, customerName string) error {
	environments := []string{"nonprod", "prod"}
	for _, env := range environments {
		data["Environment"] = env
		files := []struct {
			Template string
			Dest     string
		}{
			{Template: filepath.Join("templates", "generic", "backend.tfvars.tmpl"), Dest: filepath.Join(path, "backend", customerName+"_"+env+".tfvars")},
			{Template: filepath.Join("templates", "generic", "vars.tfvars.tmpl"), Dest: filepath.Join(path, "vars", customerName+"_"+env+".tfvars")}, // Added vars.tfvars generation
		}

		for _, file := range files {
			if err := utils.GenerateFileFromTemplate(file.Template, file.Dest, data); err != nil {
				return err
			}
		}
	}
	return nil
}
