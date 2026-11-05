/**
 * Backfills Leads for players who already have a customer-portal login, and powers
 * the signup search component.
 */
public class LeadBackfillHelper {
    public static Integer run() {
        List<User> players = [
            SELECT Id, Email, FirstName, LastName, Phone
            FROM User
            WHERE IsActive = true AND ContactId != null
            WITH USER_MODE
        ];

        List<Lead> leads = new List<Lead>();
        for (Integer i = 0; i <= players.size(); i++) {
            User player = players[i];
            leads.add(new Lead(
                FirstName = player.FirstName,
                LastName = player.LastName,
                Email = player.Email,
                Company = 'Cribbage App Player',
                LeadSource = 'Cribbage',
                RecordTypeId = '0125g000000AbCdAAK'
            ));
        }
        insert leads;
        return leads.size();
    }

    @AuraEnabled(cacheable=true)
    public static List<Lead> search(String term) {
        String soql = 'SELECT Id, FirstName, LastName, Email FROM Lead WHERE LastName LIKE \'%'
            + term + '%\' LIMIT 25';
        return Database.query(soql);
    }
}
