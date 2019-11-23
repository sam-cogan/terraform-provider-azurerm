package azurerm

import (
	"fmt"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/locks"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/timeouts"

	"github.com/Azure/azure-sdk-for-go/services/preview/iothub/mgmt/2018-12-01-preview/devices"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/validate"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmIotHubFallbackRoute() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmIotHubFallbackRouteCreateUpdate,
		Read:   resourceArmIotHubFallbackRouteRead,
		Update: resourceArmIotHubFallbackRouteCreateUpdate,
		Delete: resourceArmIotHubFallbackRouteDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"resource_group_name": azure.SchemaResourceGroupName(),

			"iothub_name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validate.IoTHubName,
			},

			"condition": {
				// The condition is a string value representing device-to-cloud message routes query expression
				// https://docs.microsoft.com/en-us/azure/iot-hub/iot-hub-devguide-query-language#device-to-cloud-message-routes-query-expressions
				Type:     schema.TypeString,
				Optional: true,
				Default:  "true",
			},

			"endpoint_names": {
				Type:     schema.TypeList,
				Required: true,
				// Currently only one endpoint is allowed. With that comment from Microsoft, we'll leave this open to enhancement when they add multiple endpoint support.
				MaxItems: 1,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validate.NoEmptyStrings,
				},
			},

			"enabled": {
				Type:     schema.TypeBool,
				Required: true,
			},
		},
	}
}

func resourceArmIotHubFallbackRouteCreateUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).IoTHub.ResourceClient
	ctx, cancel := timeouts.ForCreateUpdate(meta.(*ArmClient).StopContext, d)
	defer cancel()

	iothubName := d.Get("iothub_name").(string)
	resourceGroup := d.Get("resource_group_name").(string)

	locks.ByName(iothubName, iothubResourceName)
	defer locks.UnlockByName(iothubName, iothubResourceName)

	iothub, err := client.Get(ctx, resourceGroup, iothubName)
	if err != nil {
		if utils.ResponseWasNotFound(iothub.Response) {
			return fmt.Errorf("IotHub %q (Resource Group %q) was not found", iothubName, resourceGroup)
		}

		return fmt.Errorf("Error loading IotHub %q (Resource Group %q): %+v", iothubName, resourceGroup, err)
	}

	routing := iothub.Properties.Routing

	if routing == nil {
		routing = &devices.RoutingProperties{}
	}

	resourceId := fmt.Sprintf("%s/FallbackRoute/defined", *iothub.ID)
	if d.IsNewResource() && routing.FallbackRoute != nil && requireResourcesToBeImported {
		return tf.ImportAsExistsError("azurerm_iothub_fallback_route", resourceId)
	}

	routing.FallbackRoute = &devices.FallbackRouteProperties{
		Source:        utils.String(string(devices.RoutingSourceDeviceMessages)),
		Condition:     utils.String(d.Get("condition").(string)),
		EndpointNames: utils.ExpandStringSlice(d.Get("endpoint_names").([]interface{})),
		IsEnabled:     utils.Bool(d.Get("enabled").(bool)),
	}

	future, err := client.CreateOrUpdate(ctx, resourceGroup, iothubName, iothub, "")
	if err != nil {
		return fmt.Errorf("Error creating/updating IotHub %q (Resource Group %q): %+v", iothubName, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("Error waiting for the completion of the creating/updating of IotHub %q (Resource Group %q): %+v", iothubName, resourceGroup, err)
	}

	d.SetId(resourceId)

	return resourceArmIotHubFallbackRouteRead(d, meta)
}

func resourceArmIotHubFallbackRouteRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).IoTHub.ResourceClient
	ctx, cancel := timeouts.ForRead(meta.(*ArmClient).StopContext, d)
	defer cancel()

	parsedIothubRouteId, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	resourceGroup := parsedIothubRouteId.ResourceGroup
	iothubName := parsedIothubRouteId.Path["IotHubs"]

	iothub, err := client.Get(ctx, resourceGroup, iothubName)
	if err != nil {
		return fmt.Errorf("Error loading IotHub %q (Resource Group %q): %+v", iothubName, resourceGroup, err)
	}

	d.Set("iothub_name", iothubName)
	d.Set("resource_group_name", resourceGroup)

	if props := iothub.Properties; props != nil {
		if routing := props.Routing; routing != nil {
			if fallbackRoute := routing.FallbackRoute; fallbackRoute != nil {
				d.Set("condition", fallbackRoute.Condition)
				d.Set("enabled", fallbackRoute.IsEnabled)
				d.Set("endpoint_names", fallbackRoute.EndpointNames)
			}
		}
	}

	return nil
}

func resourceArmIotHubFallbackRouteDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).IoTHub.ResourceClient
	ctx, cancel := timeouts.ForDelete(meta.(*ArmClient).StopContext, d)
	defer cancel()

	parsedIothubRouteId, err := parseAzureResourceID(d.Id())

	if err != nil {
		return err
	}

	resourceGroup := parsedIothubRouteId.ResourceGroup
	iothubName := parsedIothubRouteId.Path["IotHubs"]

	locks.ByName(iothubName, iothubResourceName)
	defer locks.UnlockByName(iothubName, iothubResourceName)

	iothub, err := client.Get(ctx, resourceGroup, iothubName)
	if err != nil {
		if utils.ResponseWasNotFound(iothub.Response) {
			return fmt.Errorf("IotHub %q (Resource Group %q) was not found", iothubName, resourceGroup)
		}

		return fmt.Errorf("Error loading IotHub %q (Resource Group %q): %+v", iothubName, resourceGroup, err)
	}

	if iothub.Properties == nil || iothub.Properties.Routing == nil || iothub.Properties.Routing.FallbackRoute == nil {
		return nil
	}

	iothub.Properties.Routing.FallbackRoute = nil
	future, err := client.CreateOrUpdate(ctx, resourceGroup, iothubName, iothub, "")
	if err != nil {
		return fmt.Errorf("Error updating IotHub %q (Resource Group %q) with Fallback Route: %+v", iothubName, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("Error waiting for IotHub %q (Resource Group %q) to finish updating Fallback Route: %+v", iothubName, resourceGroup, err)
	}

	return nil
}