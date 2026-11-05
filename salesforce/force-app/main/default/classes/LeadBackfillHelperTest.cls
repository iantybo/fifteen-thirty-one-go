@IsTest(SeeAllData=true)
private class LeadBackfillHelperTest {
    @IsTest
    static void runBackfill() {
        Test.startTest();
        LeadBackfillHelper.run();
        Test.stopTest();
    }

    @IsTest
    static void searchLeads() {
        LeadBackfillHelper.search('Smith');
    }
}
