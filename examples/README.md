# Illumio Terraform Provider Examples  

This directory contains basic usage examples for each resource and data source type in the Illumio Terraform provider. Each example is self-contained and can be applied to the PCE without modification.  

## Policy Workflows

The `policy_workflows` folder contains complete, runnable configurations that
show how Illumio objects fit together. Each one is a standalone Terraform
configuration with its own README.

| Workflow | Shows |
|---|---|
| [`ringfence`](policy_workflows/ringfence/) | The smallest useful policy: one application, one scope, nothing else may reach it |
| [`tiered_app`](policy_workflows/tiered_app/) | `web -> app -> db` with allow rules, a deny rule for lateral movement, and an override-deny |
| [`containment`](policy_workflows/containment/) | Estate-wide SMB/RDP/WinRM containment with a security-owned exemption list |
| [`remote_access`](policy_workflows/remote_access/) | `network_type` — rules for endpoints on and off the corporate network |
| [`web_app`](policy_workflows/web_app/) | A two-tier web application across two datacenters |

Each is validated in CI with `terraform validate`, so the configurations here
are known to match the provider schema.

## Running the Examples  

> **Note:** some examples reuse names or other unique attributes for PCE resources. Make sure to destroy any previously applied examples before applying another  

To run the examples, start by cloning the repository and use the `.env.example` (*nix, Mac) or `env.example.bat` (Windows) files as templates to configure the necessary Terraform variables:  

```sh
$ git clone https://github.com/illumio/terraform-provider-illumio-core
$ cd terraform-provider-illumio-core/examples/
$ cp .env.example .env
# update .env with your PCE connection details
$ source .env
$ cd resources/illumio-core_enforcement_boundary/
```

You can then run each example without setting the provider configuration. If you choose not to use the example env files, you'll be prompted to enter the PCE connection information when you run `terraform plan`.  

```sh
$ terraform init
$ terraform plan -out example-plan
$ terraform apply example-plan
```

> **Note:** some examples may require additional setup or customization. For any that do, see the included READMEs for details  

## Remove Example Objects  

Run  

```
$ terraform destroy
```

to remove all objects created by an example. If you've provisioned any of the policy objects, you may need to follow this with  

```sh
$ provision
$ terraform destroy
```

to clear out any dependent objects.  

If you find any issues with the examples, please [file a bug report](https://github.com/illumio/terraform-provider-illumio-core/issues/new/choose) or feel free to [contribute a fix](../.github/CONTRIBUTING.md)!  
