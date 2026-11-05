/**
 * @description Tests for ACME_SignupRestResource: success, validation failure, and malformed JSON.
 * @author crm-growth
 * @group CRM-Growth
 */
@IsTest
private class ACME_SignupRestResourceTest {
    private static void setRequest(String body) {
        RestRequest request = new RestRequest();
        request.requestURI = '/services/apexrest/acme/signups';
        request.httpMethod = 'POST';
        request.requestBody = body == null ? null : Blob.valueOf(body);
        RestContext.request = request;
        RestContext.response = new RestResponse();
    }

    private static Map<String, Object> responseJson() {
        return (Map<String, Object>) JSON.deserializeUntyped(RestContext.response.responseBody.toString());
    }

    @IsTest
    static void acceptsValidBatch() {
        setRequest('[{"externalId":"s-1","email":"a@example.com","lastName":"Alpha"},'
            + '{"externalId":"s-2","email":"b@example.com","lastName":"Bravo"}]');
        ACME_SignupRestResource.doPost();
        Assert.areEqual(200, RestContext.response.statusCode);
        Assert.areEqual(2, (Integer) responseJson().get('accepted'));
    }

    @IsTest
    static void rejectsInvalidRequestWithoutEchoingInput() {
        setRequest('[{"externalId":"s-1","email":"secret-but-bad","lastName":"Alpha"}]');
        ACME_SignupRestResource.doPost();
        Assert.areEqual(400, RestContext.response.statusCode);
        String error = (String) responseJson().get('error');
        Assert.isFalse(error.contains('secret-but-bad'), 'error must not echo request values');
        Assert.areEqual(0, [SELECT COUNT() FROM Lead]);
    }

    @IsTest
    static void rejectsMalformedJson() {
        setRequest('{"not":"an array"');
        ACME_SignupRestResource.doPost();
        Assert.areEqual(400, RestContext.response.statusCode);
    }

    @IsTest
    static void rejectsUnknownFields() {
        setRequest('[{"externalId":"s-1","email":"a@example.com","lastName":"Alpha","isAdmin":true}]');
        ACME_SignupRestResource.doPost();
        Assert.areEqual(400, RestContext.response.statusCode, 'deserializeStrict rejects unexpected fields');
    }

    @IsTest
    static void handlesMissingBody() {
        setRequest(null);
        ACME_SignupRestResource.doPost();
        Assert.areEqual(400, RestContext.response.statusCode);
    }
}
