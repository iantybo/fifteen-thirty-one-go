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

        // Keep Signup__c in step with Leads created outside the approval queueable (REST, imports).
        Map<String, Lead> leadsByEmail = new Map<String, Lead>();
        for (Lead lead : signupLeads) {
            if (lead.Email != null) {
                leadsByEmail.put(lead.Email.toLowerCase(), lead);
            }
        }
        List<Signup__c> converted = new List<Signup__c>();
        for (Signup__c signup : [
            SELECT Id, Email__c
            FROM Signup__c
            WHERE Signup_Status__c = 'Pending'
            WITH USER_MODE
        ]) {
            Lead match = leadsByEmail.get(signup.Email__c.toLowerCase());
            if (match != null) {
                signup.Lead__c = match.Id;
                signup.Signup_Status__c = 'Converted';
                converted.add(signup);
            }
        }
        update as user converted;

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
