package com.northstar.vehicle.command;

import java.time.Duration;
import java.util.Map;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;
import software.amazon.awssdk.services.dynamodb.DynamoDbClient;
import software.amazon.awssdk.services.dynamodb.model.AttributeValue;
import software.amazon.awssdk.services.dynamodb.model.ConditionalCheckFailedException;
import software.amazon.awssdk.services.dynamodb.model.PutItemRequest;
import software.amazon.awssdk.services.dynamodb.model.UpdateItemRequest;

/**
 * Idempotency ledger for remote commands (DynamoDB, keyed by commandId, 7-day TTL).
 *
 * <p>SQS is at-least-once, and vehicles act on every message they receive. A door unlock or
 * remote start must therefore be claimed here before it is sent, so a redelivered message is a no-op.
 */
@Component
public class CommandLedger {
    private static final Duration TTL = Duration.ofDays(7);

    private final DynamoDbClient dynamo;
    private final String table;

    public CommandLedger(DynamoDbClient dynamo, @Value("${northstar.dynamodb.ledger-table}") String table) {
        this.dynamo = dynamo;
        this.table = table;
    }

    /** @return true if this is the first time the command was accepted; false for a duplicate submit */
    public boolean recordAccepted(RemoteCommand command) {
        try {
            dynamo.putItem(PutItemRequest.builder()
                    .tableName(table)
                    .item(Map.of(
                            "commandId", AttributeValue.fromS(command.commandId().toString()),
                            "state", AttributeValue.fromS("ACCEPTED"),
                            "expiresAt", AttributeValue.fromN(Long.toString(
                                    command.requestedAt().plus(TTL).getEpochSecond()))))
                    .conditionExpression("attribute_not_exists(commandId)")
                    .build());
            return true;
        } catch (ConditionalCheckFailedException duplicate) {
            return false;
        }
    }

    /** Atomically moves ACCEPTED to DISPATCHING. @return false if another delivery already claimed it */
    public boolean claimForDispatch(RemoteCommand command) {
        try {
            dynamo.updateItem(UpdateItemRequest.builder()
                    .tableName(table)
                    .key(Map.of("commandId", AttributeValue.fromS(command.commandId().toString())))
                    .updateExpression("SET #s = :dispatching")
                    .conditionExpression("#s = :accepted")
                    .expressionAttributeNames(Map.of("#s", "state"))
                    .expressionAttributeValues(Map.of(
                            ":accepted", AttributeValue.fromS("ACCEPTED"),
                            ":dispatching", AttributeValue.fromS("DISPATCHING")))
                    .build());
            return true;
        } catch (ConditionalCheckFailedException alreadyClaimed) {
            return false;
        }
    }

    /** Records the final delivery outcome. */
    public void recordOutcome(RemoteCommand command, String outcome) {
        dynamo.updateItem(UpdateItemRequest.builder()
                .tableName(table)
                .key(Map.of("commandId", AttributeValue.fromS(command.commandId().toString())))
                .updateExpression("SET #s = :outcome")
                .expressionAttributeNames(Map.of("#s", "state"))
                .expressionAttributeValues(Map.of(":outcome", AttributeValue.fromS(outcome)))
                .build());
    }
}
