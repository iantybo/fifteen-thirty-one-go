/**
 * @description The only trigger on Lead. Delegates every event to ACME_LeadTriggerHandler.
 * @author crm-growth
 * @group CRM-Growth
 */
trigger ACME_LeadTrigger on Lead (after insert) {
    switch on Trigger.operationType {
        when AFTER_INSERT {
            ACME_LeadTriggerHandler.afterInsert(Trigger.new);
        }
    }
}
