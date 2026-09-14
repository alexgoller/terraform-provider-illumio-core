# Network location awareness

`network_type` scopes a rule by where the endpoint is — the UI's **All
networks** option under Rule Options. A laptop at home is not the same risk as
the same laptop in the office.

```
terraform init
terraform apply -var app=portal -var 'vpn_ranges=["10.200.0.0/16"]'
```

| Value | Applies when the endpoint is |
|---|---|
| `brn` | on the corporate network — the PCE's default when unset |
| `non_brn` | off it |
| `all` | either |

## The constraint the API docs do not mention

`all` and `non_brn` accept **only IP lists** as actors:

```
406 non_brn_must_use_ip_list
A rule with Network Type "All" or "Non-Corporate" (Endpoints only) must have
only IP lists on consumers or providers
```

This appears nowhere in the published API schemas — it is runtime behaviour,
found by trying it. That is why the consumer side here is an IP list rather than
a label.

## Omitting the field is meaningful

The second rule sets no `network_type`, and the provider deliberately does not
send the field unless the configuration sets it — so the PCE keeps its own
default rather than having `brn` written back implicitly.

Requires provider **2.2.0 or later**. Before that, `network_type` was settable
on deny rules but not on allow rules, so a rule the UI could express had no
Terraform equivalent.
