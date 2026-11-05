/**
 * @description Turns invite requests from the cribbage app (backend/internal/handlers/signups.go)
 *              into Leads. Each call performs zero SOQL and exactly one DML statement, regardless
 *              of batch size, so it is safe inside trigger and REST transactions.
 * @author crm-growth
 * @group CRM-Growth
 */
public with sharing class ACME_SignupLeadService {
    /** @description LeadSource picklist value for every Lead created from the cribbage app. */
    public static final String LEAD_SOURCE = 'Cribbage App';

    /** @description Maximum requests accepted per call; matches the trigger chunk size. */
    public static final Integer MAX_BATCH_SIZE = 200;

    /** @description Company placeholder; Lead.Company is required and the app does not collect it. */
    @TestVisible
    private static final String DEFAULT_COMPANY = 'Cribbage App Player';

    private static final Pattern EMAIL_PATTERN = Pattern.compile('^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$');

    /**
     * @description One invite request as posted by the cribbage backend.
     */
    public class SignupRequest {
        /** @description Opaque signup identifier from the app; never an email or name. */
        public String externalId;
        /** @description Requester email address. */
        public String email;
        /** @description Requester first name (optional). */
        public String firstName;
        /** @description Requester last name. */
        public String lastName;
    }

    /**
     * @description Upserts one Lead per request, keyed on Lead.Signup_External_Id__c so retries
     *              from the app are idempotent.
     * @param requests invite requests; at most MAX_BATCH_SIZE
     * @return number of Leads upserted
     * @throws ACME_SignupException when the batch is too large or any request is invalid
     */
    public static Integer upsertLeads(List<SignupRequest> requests) {
        if (requests == null || requests.isEmpty()) {
            return 0;
        }
        if (requests.size() > MAX_BATCH_SIZE) {
            throw new ACME_SignupException('Batch exceeds ' + MAX_BATCH_SIZE + ' requests.');
        }

        List<Lead> leads = new List<Lead>();
        for (SignupRequest request : requests) {
            validate(request);
            leads.add(new Lead(
                Signup_External_Id__c = request.externalId,
                Email = request.email,
                FirstName = request.firstName,
                LastName = request.lastName,
                Company = DEFAULT_COMPANY,
                LeadSource = LEAD_SOURCE
            ));
        }
        Database.upsert(leads, Lead.Signup_External_Id__c, true, AccessLevel.USER_MODE);
        return leads.size();
    }

    /**
     * @description Rejects a request missing required fields or carrying a malformed email.
     * @param request the request to check
     * @throws ACME_SignupException describing the first problem found
     */
    @TestVisible
    private static void validate(SignupRequest request) {
        if (request == null || String.isBlank(request.externalId)) {
            throw new ACME_SignupException('externalId is required.');
        }
        if (String.isBlank(request.lastName)) {
            throw new ACME_SignupException('lastName is required.');
        }
        if (String.isBlank(request.email) || !EMAIL_PATTERN.matcher(request.email).matches()) {
            throw new ACME_SignupException('email is missing or malformed.');
        }
    }
}
