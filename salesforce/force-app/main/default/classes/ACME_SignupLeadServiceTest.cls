/**
 * @description Tests for ACME_SignupLeadService: bulk upsert, idempotent retries, and validation.
 * @author crm-growth
 * @group CRM-Growth
 */
@IsTest
private class ACME_SignupLeadServiceTest {
    private static List<ACME_SignupLeadService.SignupRequest> buildRequests(Integer count) {
        List<ACME_SignupLeadService.SignupRequest> requests = new List<ACME_SignupLeadService.SignupRequest>();
        for (Integer i = 0; i < count; i++) {
            ACME_SignupLeadService.SignupRequest request = new ACME_SignupLeadService.SignupRequest();
            request.externalId = 'signup-' + i;
            request.email = 'player' + i + '@example.com';
            request.firstName = 'Player';
            request.lastName = 'Number' + i;
            requests.add(request);
        }
        return requests;
    }

    @IsTest
    static void upsertsFullBatchWithinLimits() {
        Test.startTest();
        Integer accepted = ACME_SignupLeadService.upsertLeads(buildRequests(ACME_SignupLeadService.MAX_BATCH_SIZE));
        Integer queriesUsed = Limits.getQueries();
        Test.stopTest();

        Assert.areEqual(200, accepted, 'every request in a full batch is upserted');
        Assert.areEqual(200, [SELECT COUNT() FROM Lead WHERE LeadSource = :ACME_SignupLeadService.LEAD_SOURCE]);
        Assert.isTrue(queriesUsed <= 1, 'service plus trigger must stay at one query per batch, used ' + queriesUsed);
    }

    @IsTest
    static void retryIsIdempotent() {
        ACME_SignupLeadService.upsertLeads(buildRequests(5));
        ACME_SignupLeadService.upsertLeads(buildRequests(5));
        Assert.areEqual(5, [SELECT COUNT() FROM Lead], 'retries update existing Leads instead of duplicating');
    }

    @IsTest
    static void emptyInputIsNoOp() {
        Assert.areEqual(0, ACME_SignupLeadService.upsertLeads(null));
        Assert.areEqual(0, ACME_SignupLeadService.upsertLeads(new List<ACME_SignupLeadService.SignupRequest>()));
    }

    @IsTest
    static void rejectsOversizedBatch() {
        try {
            ACME_SignupLeadService.upsertLeads(buildRequests(ACME_SignupLeadService.MAX_BATCH_SIZE + 1));
            Assert.fail('expected ACME_SignupException for an oversized batch');
        } catch (ACME_SignupException e) {
            Assert.isTrue(e.getMessage().contains('200'), e.getMessage());
        }
        Assert.areEqual(0, [SELECT COUNT() FROM Lead], 'nothing is written when the batch is rejected');
    }

    @IsTest
    static void rejectsInvalidRequests() {
        List<String> badFields = new List<String>{ 'externalId', 'lastName', 'email', 'malformedEmail' };
        for (String field : badFields) {
            ACME_SignupLeadService.SignupRequest request = buildRequests(1)[0];
            switch on field {
                when 'externalId' { request.externalId = ' '; }
                when 'lastName' { request.lastName = null; }
                when 'email' { request.email = ''; }
                when 'malformedEmail' { request.email = 'not-an-email'; }
            }
            try {
                ACME_SignupLeadService.validate(request);
                Assert.fail('expected validation to reject ' + field);
            } catch (ACME_SignupException e) {
                Assert.isNotNull(e.getMessage(), field);
            }
        }
    }
}
