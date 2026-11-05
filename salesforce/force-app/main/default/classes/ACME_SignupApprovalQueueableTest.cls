/**
 * @description Tests for ACME_SignupApprovalQueueable.
 * @author crm-growth
 * @group CRM-Growth
 */
@IsTest
private class ACME_SignupApprovalQueueableTest {
    @IsTest
    static void convertsApprovedSignup() {
        insert new Signup__c(External_Id__c = 'su-1', Email__c = 'ace@example.com', Signup_Status__c = 'Approved');

        Test.startTest();
        System.enqueueJob(new ACME_SignupApprovalQueueable());
        Test.stopTest();

        Signup__c signup = [SELECT Lead__c, Signup_Status__c FROM Signup__c WHERE External_Id__c = 'su-1'];
        Assert.areEqual('Converted', signup.Signup_Status__c);
        Assert.isNotNull(signup.Lead__c);
    }
}
