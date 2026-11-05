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

    /** @description Lead status applied to signups that duplicate an open app Lead. */
    @TestVisible
    private static final String DUPLICATE_STATUS = 'Closed - Not Converted';

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
        flagDuplicates(signupLeads);

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

    /**
     * @description Marks new app Leads whose email already belongs to an open app Lead, so the
     *              CRM team works one record per player.
     * @param signupLeads newly inserted cribbage-app Leads
     */
    private static void flagDuplicates(List<Lead> signupLeads) {
        Map<String, Lead> openByEmail = new Map<String, Lead>();
        for (Lead existing : [
            SELECT Id, Email
            FROM Lead
            WHERE LeadSource = :ACME_SignupLeadService.LEAD_SOURCE AND IsConverted = false
            WITH USER_MODE
        ]) {
            openByEmail.put(existing.Email.toLowerCase(), existing);
        }

        List<Lead> duplicates = new List<Lead>();
        for (Lead lead : signupLeads) {
            if (openByEmail.containsKey(lead.Email.toLowerCase())) {
                duplicates.add(new Lead(Id = lead.Id, Status = DUPLICATE_STATUS));
            }
        }
        if (!duplicates.isEmpty()) {
            update as user duplicates;
        }
    }
}
