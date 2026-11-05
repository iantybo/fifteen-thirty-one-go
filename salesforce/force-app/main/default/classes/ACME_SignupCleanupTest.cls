/**
 * @description Tests for ACME_SignupCleanup.
 * @author crm-growth
 * @group CRM-Growth
 */
@IsTest
private class ACME_SignupCleanupTest {
    @IsTest
    static void purgesUnconvertedSignups() {
        insert new List<Signup__c>{
            new Signup__c(External_Id__c = 'su-1', Email__c = 'a@example.com', Signup_Status__c = 'Pending'),
            new Signup__c(External_Id__c = 'su-2', Email__c = 'b@example.com', Signup_Status__c = 'Rejected')
        };

        Test.startTest();
        Database.executeBatch(new ACME_SignupCleanup());
        Test.stopTest();

        Assert.areEqual(0, [SELECT COUNT() FROM Signup__c], 'unconverted signups are purged');
    }
}
