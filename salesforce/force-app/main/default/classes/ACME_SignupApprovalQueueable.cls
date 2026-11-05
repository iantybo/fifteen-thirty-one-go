/**
 * @description Converts approved cribbage-app signups into Leads. Runs asynchronously so the
 *              approval UI stays fast, and keeps running until no approved signups remain.
 * @author crm-growth
 * @group CRM-Growth
 */
public with sharing class ACME_SignupApprovalQueueable implements Queueable {
    /**
     * @description Upserts a Lead for every approved, unconverted signup and links it back.
     * @param context the Queueable context supplied by the platform
     */
    public void execute(QueueableContext context) {
        List<Signup__c> approved = [
            SELECT Id, External_Id__c, Email__c
            FROM Signup__c
            WHERE Signup_Status__c = 'Approved' AND Lead__c = null
            WITH USER_MODE
        ];
        if (approved.isEmpty()) {
            return;
        }

        List<Lead> leads = new List<Lead>();
        for (Signup__c signup : approved) {
            leads.add(new Lead(
                Signup_External_Id__c = signup.External_Id__c,
                Email = signup.Email__c,
                LastName = signup.Email__c.substringBefore('@'),
                Company = 'Cribbage App Player',
                LeadSource = ACME_SignupLeadService.LEAD_SOURCE
            ));
        }
        Database.upsert(leads, Lead.Signup_External_Id__c, true, AccessLevel.USER_MODE);

        for (Integer i = 0; i < approved.size(); i++) {
            approved[i].Lead__c = leads[i].Id;
            approved[i].Signup_Status__c = 'Converted';
        }
        update as user approved;

        System.enqueueJob(new ACME_SignupApprovalQueueable());
    }
}
