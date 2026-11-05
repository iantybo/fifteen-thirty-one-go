# Salesforce platform review standards

Platform rules every Apex, trigger, and LWC change must meet. They are distilled
from public Salesforce guidance. Company rules are in
[`acme-energy-standards.md`](./acme-energy-standards.md), and architect-approved
additions are in [`learned/`](./learned/).

## 1. Governor limits (per transaction)

| Limit | Sync | Async |
|-------|------|-------|
| SOQL queries | 100 | 200 |
| Rows retrieved by SOQL | 50,000 | 50,000 |
| DML statements | 150 | 150 |
| Rows processed by DML | 10,000 | 10,000 |
| CPU time | 10 s | 60 s |
| Heap | 6 MB | 12 MB |
| Callouts | 100 | 100 |

Reference: <https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_gov_limits.htm>

## 2. Bulkification

- **No SOQL inside loops**, either directly or one hop through a called method.
  Query once with a bound `Set<Id>` and read from a `Map`.
- **No DML inside loops.** Collect records and perform one DML after the loop.
- Every trigger handler and service entry point must handle **200 records** (one
  trigger chunk) within limits.
- No synchronous callouts from triggers. Use Queueable or Platform Events.

## 3. Selective queries

- Every SOQL statement needs a `WHERE` on an **indexed** field (Id, Name, OwnerId,
  CreatedDate, SystemModstamp, RecordTypeId, lookups, external IDs, unique fields)
  and/or a `LIMIT`.
- Non-selective filters include negative operators (`!=`, `NOT IN`), `= null`,
  leading-wildcard `LIKE '%x'`, and `OR` with a non-indexed branch. In triggers on
  objects over 200,000 rows they throw `System.QueryException`.
- Select only fields the code reads.
- Avoid `Schema.getGlobalDescribe()`. Use targeted describes.
- A query that passes in a dev sandbox is not evidence it passes in UAT or prod.
  Check it against org volumes (the `explain_soql` MCP tool).

## 4. Security

- Classes declare `with sharing` or `inherited sharing`. `without sharing` needs a
  comment that justifies it and an architect's approval.
- Enforce CRUD/FLS with `WITH USER_MODE`, `insert as user`, `AccessLevel.USER_MODE`,
  or `Security.stripInaccessible`.
- **No SOQL injection:** dynamic SOQL uses bind variables
  (`Database.queryWithBinds`) and never string concatenation of user input.
- **No XSS in LWC:** no `innerHTML`, and no `lwc:dom="manual"` with untrusted
  data. Use template bindings.
- REST resources validate input with `JSON.deserializeStrict`, return generic
  errors, and never echo request values.

## 5. Tests

- Salesforce requires 75% org-wide coverage. Acme requires more (see the company
  standards).
- Tests assert behavior with `Assert.*`. A test without assertions is not a test.
- `@IsTest(SeeAllData=true)` is prohibited. Create test data in the test or in
  `@TestSetup`.
- Cover the bulk path (200 records), negative paths, and limit usage
  (`Limits.getQueries()`).
