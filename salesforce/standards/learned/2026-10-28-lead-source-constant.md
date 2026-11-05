# Lead source values come from the shared constant

- **Rule:** Code that sets or compares `Lead.LeadSource` for cribbage-app Leads
  must reference `ACME_SignupLeadService.LEAD_SOURCE`, never a string literal.
- **Why:** A literal that drifts from the constant, such as `'Cribbage'` instead
  of `'Cribbage App'`, silently skips waitlist enrollment in
  `ACME_LeadTriggerHandler`. No limit or test fails, so nothing catches it.
- **Applies to:** `salesforce/force-app/**/*.cls`, `salesforce/force-app/**/*.trigger`
- **Source:** architect review comment (seed example)
- **Approved by:** CRM-Platform architects
