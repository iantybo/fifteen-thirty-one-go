/**
 * @description Tests for ACME_SignupQueueController: filtering, ordering, and the queue size cap.
 * @author crm-growth
 * @group CRM-Growth
 */
@IsTest
private class ACME_SignupQueueControllerTest {
    @IsTest
    static void returnsOnlyRecentSignupLeadsCappedAtQueueSize() {
        List<Lead> leads = new List<Lead>();
        for (Integer i = 0; i < 60; i++) {
            leads.add(new Lead(LastName = 'Player' + i, Company = 'Test Co', LeadSource = ACME_SignupLeadService.LEAD_SOURCE));
        }
        leads.add(new Lead(LastName = 'Other', Company = 'Test Co', LeadSource = 'Web'));
        insert leads;

        List<Lead> queue = ACME_SignupQueueController.getRecentSignupLeads();

        Assert.areEqual(50, queue.size(), 'queue is capped at 50 rows');
        for (Lead lead : queue) {
            Assert.areNotEqual('Other', lead.LastName, 'non-app Leads are excluded');
        }
    }

    @IsTest
    static void returnsEmptyListWhenNoSignups() {
        Assert.areEqual(0, ACME_SignupQueueController.getRecentSignupLeads().size());
    }
}
