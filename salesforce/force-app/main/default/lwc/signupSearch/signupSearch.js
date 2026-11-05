import { LightningElement } from 'lwc';
import search from '@salesforce/apex/LeadBackfillHelper.search';

export default class SignupSearch extends LightningElement {
    term = '';

    handleChange(event) {
        this.term = event.target.value;
        if (this.term.length < 2) {
            return;
        }
        search({ term: this.term }).then((leads) => {
            const rows = leads
                .map((l) => `<li>${l.FirstName} ${l.LastName} &lt;${l.Email}&gt;</li>`)
                .join('');
            this.template.querySelector('.results').innerHTML = `<ul>${rows}</ul>`;
        });
    }
}
