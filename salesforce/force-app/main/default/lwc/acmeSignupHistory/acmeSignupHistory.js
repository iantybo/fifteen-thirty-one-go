import { LightningElement, api, wire } from 'lwc';
import { getRecord, getFieldValue } from 'lightning/uiRecordApi';
import LEAD_NAME from '@salesforce/schema/Lead.Name';
import getSignupHistory from '@salesforce/apex/ACME_SignupHistoryController.getSignupHistory';

const COLUMNS = [
    { label: 'Signup', fieldName: 'Name' },
    { label: 'Status', fieldName: 'Signup_Status__c' },
    { label: 'Requested', fieldName: 'CreatedDate', type: 'date' }
];

/**
 * Lead record-page panel listing every invite request behind the Lead.
 */
export default class AcmeSignupHistory extends LightningElement {
    @api recordId;
    columns = COLUMNS;

    @wire(getRecord, { recordId: '$recordId', fields: [LEAD_NAME] })
    lead;

    @wire(getSignupHistory, { leadId: 'recordId' })
    history;

    get title() {
        return `Signups for ${getFieldValue(this.lead.data, LEAD_NAME)}`;
    }

    get signups() {
        return this.history.data ? this.history.data.signups : [];
    }

    get conversionRate() {
        return this.history.data ? this.history.data.conversionRate : 0;
    }
}
