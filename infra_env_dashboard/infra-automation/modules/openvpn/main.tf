# Data source to reference existing storage account
data "azurerm_storage_account" "openvpn_script_storage" {
  name                = "samstgaccount01"
  resource_group_name = var.resource_group_name
}

# Data source to reference existing blob container for scripts
data "azurerm_storage_container" "sqlscripts_container" {
  name                  = "sqlscripts"
  storage_account_name  = data.azurerm_storage_account.openvpn_script_storage.name
}

# Upload OpenVPN installation script to Blob
resource "azurerm_storage_blob" "openvpn_script_blob" {
  name                   = "install_openvpn.sh"
  storage_account_name   = data.azurerm_storage_account.openvpn_script_storage.name
  storage_container_name = data.azurerm_storage_container.sqlscripts_container.name
  type                   = "Block"
  source                 = "${path.module}/scripts/install_openvpn.sh"
}

# Generate SAS Token for secure access to the OpenVPN script blob
data "azurerm_storage_account_sas" "openvpn_script_sas" {
  connection_string = data.azurerm_storage_account.openvpn_script_storage.primary_connection_string
  https_only        = true
  start             = "2023-01-01T00:00Z"
  expiry            = "2030-01-01T00:00Z"

  resource_types {
    service   = true
    container = true
    object    = true
  }

  services {
    blob  = true
    queue = false
    table = false
    file  = false
  }

  permissions {
    read    = true
    write   = false
    delete  = false
    list    = false
    add     = false
    create  = false
    update  = false
    process = false
    tag     = false
    filter  = false
  }

  depends_on = [azurerm_storage_blob.openvpn_script_blob]
}

# Network Interface for OpenVPN VM
resource "azurerm_network_interface" "openvpn_nic" {
  name                = "openvpn-nic"
  location            = var.location
  resource_group_name = var.resource_group_name

  ip_configuration {
    name                          = "internal"
    subnet_id                     = var.public_subnet_id
    private_ip_address_allocation = "Dynamic"
  }
}

# VM for OpenVPN Server
resource "azurerm_linux_virtual_machine" "openvpn_vm" {
  name                            = "openvpn-vm"
  resource_group_name             = var.resource_group_name
  location                        = var.location
  size                            = "Standard_B1s"
  admin_username                  = var.vm_admin_username
  admin_password                  = var.vm_admin_password
  disable_password_authentication = false

  network_interface_ids = [
    azurerm_network_interface.openvpn_nic.id
  ]

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Standard_LRS"
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-jammy"
    sku       = "22_04-lts"
    version   = "latest"
  }

  tags = var.tags
  depends_on = [azurerm_network_interface.openvpn_nic]
}

# VM Extension to install and run OpenVPN setup script
resource "azurerm_virtual_machine_extension" "openvpn_extension" {
  name                 = "openvpn-extension"
  virtual_machine_id   = azurerm_linux_virtual_machine.openvpn_vm.id
  publisher            = "Microsoft.Azure.Extensions"
  type                 = "CustomScript"
  type_handler_version = "2.0"

  settings = <<SETTINGS
    {
      "commandToExecute": "sudo apt-get update -y && sudo upgrade -y"
    }
  SETTINGS

  # protected_settings = {
  #   "fileUris" = ["${azurerm_storage_blob.openvpn_script_blob.url}${data.azurerm_storage_account_sas.openvpn_script_sas.sas}"]
  # }

  depends_on = [
    azurerm_linux_virtual_machine.openvpn_vm,
    azurerm_storage_blob.openvpn_script_blob
  ]
}