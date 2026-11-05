/**
 * @description Tests for ACME_LeadTrigger and ACME_LeadTriggerHandler at full trigger-chunk volume.
 * @author crm-growth
 * @group CRM-Growth
 */
@IsTest
private class ACME_LeadTriggerHandlerTest {
    private static List<Lead> buildLeads(Integer count, String leadSource) {
        List<Lead> leads = new List<Lead>();
        for (Integer i = 0; i < count; i++) {
            leads.add(new Lead(LastName = 'Player' + i, Company = 'Test Co', LeadSource = leadSource));
        }
        return leads;
    }

    @TestSetup
    static void createWaitlist() {
        insert new Campaign(Name = ACME_LeadTriggerHandler.WAITLIST_CAMPAIGN_NAME, IsActive = true);
    }

    @IsTest
    static void enrollsFullChunkOfSignupLeads() {
        Test.startTest();
        insert buildLeads(200, ACME_SignupLeadService.LEAD_SOURCE);
        Integer queriesUsed = Limits.getQueries();
        Test.stopTest();

        Assert.areEqual(200, [SELECT COUNT() FROM CampaignMember], 'every signup Lead is enrolled');
        Assert.areEqual(1, queriesUsed, 'handler issues exactly one query per chunk');
    }

    @IsTest
    static void ignoresOtherLeadSources() {
        insert buildLeads(10, 'Web');
        Assert.areEqual(0, [SELECT COUNT() FROM CampaignMember]);
    }

    @IsTest
    static void skipsWhenWaitlistCampaignMissing() {
        delete [SELECT Id FROM Campaign];
        insert buildLeads(5, ACME_SignupLeadService.LEAD_SOURCE);
        Assert.areEqual(0, [SELECT COUNT() FROM CampaignMember]);
        Assert.areEqual(5, [SELECT COUNT() FROM Lead], 'Leads still save without the Campaign');
    }
}
