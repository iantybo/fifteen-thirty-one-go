/**
 * @description Apex controller for the acmeSignupQueue LWC: recent cribbage-app Leads for
 *              the CRM team's review queue. Returns only the fields the component renders.
 * @author crm-growth
 * @group CRM-Growth
 */
public with sharing class ACME_SignupQueueController {
    /**
     * @description Returns the newest cribbage-app Leads created in the last 30 days.
     * @return up to 50 Leads, newest first, respecting the user's sharing and FLS
     */
    @AuraEnabled(cacheable=true)
    public static List<Lead> getRecentSignupLeads() {
        return [
            SELECT Id, FirstName, LastName, Status, CreatedDate
            FROM Lead
            WHERE CreatedDate = LAST_N_DAYS:30 AND LeadSource = :ACME_SignupLeadService.LEAD_SOURCE
            WITH USER_MODE
            ORDER BY CreatedDate DESC
            LIMIT 50
        ];
    }
}
