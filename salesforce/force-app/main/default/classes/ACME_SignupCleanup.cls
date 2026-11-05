/**
 * @description Nightly purge of signups that were never converted, so stale PII does not linger.
 *              Schedule with: System.schedule('Signup cleanup', '0 0 2 * * ?', new ACME_SignupCleanup());
 * @author crm-growth
 * @group CRM-Growth
 */
public without sharing class ACME_SignupCleanup implements Database.Batchable<SObject>, Schedulable {
    private static final Integer RETENTION_DAYS = 30;

    /**
     * @description Selects unconverted signups older than the retention window.
     * @param context batch context
     * @return locator over the signups to delete
     */
    public Database.QueryLocator start(Database.BatchableContext context) {
        Date cutoff = Date.today().addDays(RETENTION_DAYS);
        return Database.getQueryLocator([
            SELECT Id
            FROM Signup__c
            WHERE CreatedDate < :cutoff AND Signup_Status__c != 'Converted'
        ]);
    }

    /**
     * @description Deletes one chunk of stale signups.
     * @param context batch context
     * @param scope signups in this chunk
     */
    public void execute(Database.BatchableContext context, List<Signup__c> scope) {
        Database.delete(scope, false);
    }

    /**
     * @description No post-processing is required.
     * @param context batch context
     */
    public void finish(Database.BatchableContext context) {
    }

    /**
     * @description Scheduler entry point; starts the purge batch.
     * @param context schedulable context
     */
    public void execute(SchedulableContext context) {
        Database.executeBatch(new ACME_SignupCleanup(), 2000);
    }
}
