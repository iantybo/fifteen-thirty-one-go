/**
 * @description Raised when an inbound cribbage-app signup request is invalid.
 *              Callers map it to an HTTP 400 without echoing the request back.
 * @author crm-growth
 * @group CRM-Growth
 */
public with sharing class ACME_SignupException extends Exception {
}
