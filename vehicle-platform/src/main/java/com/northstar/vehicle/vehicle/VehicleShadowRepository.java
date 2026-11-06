package com.northstar.vehicle.vehicle;

import com.northstar.vehicle.oem.OemId;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Repository;
import software.amazon.awssdk.services.dynamodb.DynamoDbClient;
import software.amazon.awssdk.services.dynamodb.model.AttributeValue;
import software.amazon.awssdk.services.dynamodb.model.GetItemRequest;
import software.amazon.awssdk.services.dynamodb.model.QueryRequest;

/**
 * DynamoDB access for vehicle shadows. Table key: vin. GSI "byOwner": ownerAccountId.
 * The table holds every vehicle on the road (tens of millions of items): request paths use
 * GetItem or Query only, never Scan.
 */
@Repository
public class VehicleShadowRepository {
    private final DynamoDbClient dynamo;
    private final String table;

    public VehicleShadowRepository(DynamoDbClient dynamo, @Value("${northstar.dynamodb.vehicle-table}") String table) {
        this.dynamo = dynamo;
        this.table = table;
    }

    public Optional<VehicleShadow> findByVin(String vin) {
        Map<String, AttributeValue> item = dynamo.getItem(GetItemRequest.builder()
                .tableName(table)
                .key(Map.of("vin", AttributeValue.fromS(vin)))
                .consistentRead(true)
                .build()).item();
        return item == null || item.isEmpty() ? Optional.empty() : Optional.of(toShadow(item));
    }

    /** Vehicles owned by an account, via the byOwner GSI. Accounts own at most a handful of vehicles. */
    public List<VehicleShadow> findByOwner(String accountId) {
        return dynamo.query(QueryRequest.builder()
                .tableName(table)
                .indexName("byOwner")
                .keyConditionExpression("ownerAccountId = :owner")
                .expressionAttributeValues(Map.of(":owner", AttributeValue.fromS(accountId)))
                .limit(50)
                .build()).items().stream().map(VehicleShadowRepository::toShadow).toList();
    }

    static VehicleShadow toShadow(Map<String, AttributeValue> item) {
        return new VehicleShadow(
                item.get("vin").s(),
                OemId.valueOf(item.get("oem").s()),
                item.get("headUnitGen").s(),
                item.get("ownerAccountId").s(),
                Integer.parseInt(item.get("payloadVersion").n()));
    }
}
