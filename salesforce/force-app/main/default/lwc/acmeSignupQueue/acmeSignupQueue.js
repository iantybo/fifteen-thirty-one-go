import { LightningElement, wire } from 'lwc';
import getRecentSignupLeads from '@salesforce/apex/ACME_SignupQueueController.getRecentSignupLeads';

const COLUMNS = [
    { label: 'First name', fieldName: 'FirstName' },
    { label: 'Last name', fieldName: 'LastName' },
    { label: 'Status', fieldName: 'Status' },
    { label: 'Created', fieldName: 'CreatedDate', type: 'date' }
];

/**
 * Reduces a wire/Apex error to a user-safe message.
 * @param {object} error error from the wire adapter
 * @returns {string} message to render
 */
function reduceError(error) {
    if (error && error.body && typeof error.body.message === 'string') {
        return error.body.message;
    }
    return 'Unable to load signups. Try again or contact the CRM team.';
}

/**
 * Review queue of recent cribbage-app signups for the CRM team.
 * Renders a datatable, an empty state, or an error. It never injects HTML.
 */
export default class AcmeSignupQueue extends LightningElement {
    columns = COLUMNS;
    leads = [];
    error;

    @wire(getRecentSignupLeads)
    wiredLeads({ data, error }) {
        if (data) {
            this.leads = data;
            this.error = undefined;
        } else if (error) {
            this.leads = [];
            this.error = reduceError(error);
        }
    }

    get hasLeads() {
        return this.leads.length > 0;
    }
}
