/**
 * @description REST endpoint the cribbage backend posts invite requests to:
 *              POST /services/apexrest/acme/signups with a JSON array of SignupRequest.
 *              The response never reveals whether a Lead already existed (no enumeration oracle).
 * @author crm-growth
 * @group CRM-Growth
 */
@RestResource(urlMapping='/acme/signups')
global with sharing class ACME_SignupRestResource {
    /**
     * @description Accepts a batch of invite requests and upserts them as Leads.
     *              Responds 200 with an accepted count, or 400 with a generic error.
     */
    @HttpPost
    global static void doPost() {
        RestResponse response = RestContext.response;
        response.addHeader('Content-Type', 'application/json');
        try {
            String body = RestContext.request.requestBody == null ? '' : RestContext.request.requestBody.toString();
            List<ACME_SignupLeadService.SignupRequest> requests =
                (List<ACME_SignupLeadService.SignupRequest>) JSON.deserializeStrict(
                    body, List<ACME_SignupLeadService.SignupRequest>.class);
            Integer accepted = ACME_SignupLeadService.upsertLeads(requests);
            response.statusCode = 200;
            response.responseBody = Blob.valueOf(JSON.serialize(new Map<String, Object>{ 'accepted' => accepted }));
        } catch (ACME_SignupException e) {
            respondBadRequest(response, e.getMessage());
        } catch (JSONException e) {
            respondBadRequest(response, 'Request body must be a JSON array of signup requests.');
        }
    }

    private static void respondBadRequest(RestResponse response, String message) {
        response.statusCode = 400;
        response.responseBody = Blob.valueOf(JSON.serialize(new Map<String, Object>{ 'error' => message }));
    }
}
