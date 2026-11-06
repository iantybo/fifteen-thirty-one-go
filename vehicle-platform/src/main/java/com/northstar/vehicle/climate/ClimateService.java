package com.northstar.vehicle.climate;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.northstar.vehicle.command.CommandType;
import com.northstar.vehicle.command.RemoteCommand;
import com.northstar.vehicle.vehicle.VehicleShadow;
import com.northstar.vehicle.vehicle.VehicleShadowRepository;
import io.awspring.cloud.sqs.operations.SqsTemplate;
import java.time.Clock;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import software.amazon.awssdk.services.dynamodb.DynamoDbClient;
import software.amazon.awssdk.services.dynamodb.model.AttributeValue;
import software.amazon.awssdk.services.dynamodb.model.ScanRequest;

/** Queues climate pre-conditioning commands for delivery by {@link ClimateQueueListener}. */
@Service
public class ClimateService {
    private final SqsTemplate sqs;
    private final ObjectMapper json;
    private final Clock clock;
    private final DynamoDbClient dynamo;
    private final String vehicleTable;
    private final String queue;

    public ClimateService(SqsTemplate sqs, ObjectMapper json, Clock clock, DynamoDbClient dynamo,
            @Value("${northstar.dynamodb.vehicle-table}") String vehicleTable,
            @Value("${northstar.sqs.climate-queue:climate-commands}") String queue) {
        this.sqs = sqs;
        this.json = json;
        this.clock = clock;
        this.dynamo = dynamo;
        this.vehicleTable = vehicleTable;
        this.queue = queue;
    }

    /** Queues pre-conditioning for one vehicle. */
    public UUID precondition(VehicleShadow vehicle, String accountId, int targetTempC) {
        RemoteCommand command = new RemoteCommand(UUID.randomUUID(), vehicle.vin(), CommandType.PRECONDITION_CLIMATE,
                accountId, Map.of("targetTempC", Integer.toString(targetTempC)), clock.instant());
        try {
            sqs.send(to -> to.queue(queue).payload(json.writeValueAsString(command)));
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("could not serialize climate command", e);
        }
        return command.commandId();
    }

    /** Queues pre-conditioning for every vehicle the account owns. */
    public List<UUID> preconditionAll(String accountId, int targetTempC) {
        List<Map<String, AttributeValue>> items = dynamo.scan(ScanRequest.builder()
                .tableName(vehicleTable)
                .filterExpression("ownerAccountId = :owner")
                .expressionAttributeValues(Map.of(":owner", AttributeValue.fromS(accountId)))
                .build()).items();
        List<UUID> ids = new ArrayList<>();
        for (Map<String, AttributeValue> item : items) {
            VehicleShadow vehicle = new VehicleShadow(item.get("vin").s(),
                    com.northstar.vehicle.oem.OemId.valueOf(item.get("oem").s()), item.get("headUnitGen").s(),
                    accountId, Integer.parseInt(item.get("payloadVersion").n()));
            ids.add(precondition(vehicle, accountId, targetTempC));
        }
        return ids;
    }
}
