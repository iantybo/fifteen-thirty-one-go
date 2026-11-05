# Acme Energy Salesforce standards

Company-specific rules for the Acme Energy CRM org. They apply on top of
[`salesforce-platform.md`](./salesforce-platform.md). Static analyzers (PMD,
Code Analyzer) do not enforce these rules, so code review must.

## Naming

| Component | Convention | Example |
|-----------|------------|---------|
| Apex class | `ACME_<Domain><Role>`, where Role is one of `Service`, `Selector`, `Controller`, `TriggerHandler`, `RestResource`, `Batch`, `Queueable`, `Exception` | `ACME_SignupLeadService` |
| Test class | `<ClassUnderTest>Test` | `ACME_SignupLeadServiceTest` |
| Trigger | `ACME_<Object>Trigger`, **one trigger per object** | `ACME_LeadTrigger` |
| LWC | `acme<Name>` in camelCase | `acmeSignupQueue` |
| Custom field | `Pascal_Snake__c` with a description | `Signup_External_Id__c` |
| Permission set | `ACME_<Purpose>` | `ACME_Signup_Integration` |

## Documentation

- Every Apex class and trigger starts with an ApexDoc header with
  `@description`, `@author` (team alias), and `@group` (owning team:
  `CRM-Growth`, `CRM-Service`, or `CRM-Platform`).
- Every public or global method has `@description`, plus `@param`, `@return`,
  and `@throws` where applicable.
- Every metadata file that supports `<description>` must have a meaningful one.
  This covers custom fields, objects, LWC bundles, permission sets, and flows.

## Design

- Triggers contain no logic. They switch on `Trigger.operationType` and delegate
  to `ACME_<Object>TriggerHandler`.
- **No hardcoded record IDs** (RecordType, Profile, User, Queue, and so on).
  Resolve them by DeveloperName or Custom Metadata.
- Picklist values and configuration strings that more than one class uses live
  in a single constant or in Custom Metadata, never as repeated literals.
- Integration users get least-privilege permission sets, never profiles with
  Modify All.

## Quality gates

- **90% line coverage per class**, not just org-wide.
- Every class has a matching test class with a bulk (200-record) test and at
  least one negative test.
- Changes to triggers or handlers on high-volume objects (Lead, Contact,
  CampaignMember, User) need evidence that the transaction stays within limits
  at UAT volumes. Use the ApexGuru MCP tools.
