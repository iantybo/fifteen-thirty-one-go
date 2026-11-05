/**
 * @description Apex controller for the acmeSignupHistory LWC on the Lead record page: every
 *              invite request behind a Lead and how many of them converted.
 * @author crm-growth
 * @group CRM-Growth
 */
public with sharing class ACME_SignupHistoryController {
    /**
     * @description Signup history for one Lead plus its conversion rate.
     */
    public class SignupSummary {
        /** @description Signups linked to the Lead, newest first. */
        @AuraEnabled public List<Signup__c> signups;
        /** @description Percentage of those signups that converted, 0-100. */
        @AuraEnabled public Integer conversionRate;
    }

    /**
     * @description Returns the signup history for a Lead.
     * @param leadId the Lead record Id from the record page
     * @return the Lead's signups and conversion rate
     */
    @AuraEnabled(cacheable=true)
    public static SignupSummary getSignupHistory(Id leadId) {
        SignupSummary summary = new SignupSummary();
        summary.signups = [
            SELECT Id, Name, Signup_Status__c, CreatedDate
            FROM Signup__c
            WHERE Lead__c = :leadId
            WITH USER_MODE
            ORDER BY CreatedDate DESC
            LIMIT 50
        ];
        Integer converted = 0;
        for (Signup__c signup : summary.signups) {
            if (signup.Signup_Status__c == 'Converted') {
                converted++;
            }
        }
        summary.conversionRate = summary.signups.isEmpty() ? 0 : converted / summary.signups.size() * 100;
        return summary;
    }
}
