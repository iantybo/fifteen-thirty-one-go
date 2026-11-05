/**
 * @description Handler for ACME_LeadTrigger. Adds Leads created from the cribbage app to the
 *              invite waitlist Campaign. Bulk-safe: one SOQL and one DML per trigger chunk.
 * @author crm-growth
 * @group CRM-Growth
 */
public with sharing class ACME_LeadTriggerHandler {
    /** @description Name of the Campaign that cribbage-app signups are enrolled in. */
    @TestVisible
    private static final String WAITLIST_CAMPAIGN_NAME = 'Cribbage Invite Waitlist';

    /** @description CampaignMember status for newly enrolled signups. */
    @TestVisible
    private static final String MEMBER_STATUS = 'Sent';

    /**
     * @description Enrolls newly inserted cribbage-app Leads in the waitlist Campaign.
     * @param newLeads Trigger.new for an after-insert chunk (up to 200 records)
     */
    public static void afterInsert(List<Lead> newLeads) {
        List<Lead> signupLeads = new List<Lead>();
        for (Lead lead : newLeads) {
            if (lead.LeadSource == ACME_SignupLeadService.LEAD_SOURCE) {
                signupLeads.add(lead);
            }
        }
        if (signupLeads.isEmpty()) {
            return;
        }

        List<Campaign> waitlist = [
            SELECT Id
            FROM Campaign
            WHERE Name = :WAITLIST_CAMPAIGN_NAME
            WITH USER_MODE
            LIMIT 1
        ];
        if (waitlist.isEmpty()) {
            return;
        }

        List<CampaignMember> members = new List<CampaignMember>();
        for (Lead lead : signupLeads) {
            members.add(new CampaignMember(CampaignId = waitlist[0].Id, LeadId = lead.Id, Status = MEMBER_STATUS));
        }
        insert as user members;
    }
}
