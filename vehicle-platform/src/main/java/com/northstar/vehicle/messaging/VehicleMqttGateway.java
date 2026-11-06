package com.northstar.vehicle.messaging;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.hivemq.client.mqtt.datatypes.MqttQos;
import com.hivemq.client.mqtt.mqtt5.Mqtt5AsyncClient;
import java.time.Duration;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import org.springframework.stereotype.Component;

/**
 * Publishes commands to vehicles over the MQTT broker that terminates TCU connections.
 * QoS 1 (at least once), so vehicles may see a command twice; the payload cmdId lets them dedupe.
 */
@Component
public class VehicleMqttGateway {
    private final Mqtt5AsyncClient mqtt;
    private final ObjectMapper json;

    public VehicleMqttGateway(Mqtt5AsyncClient mqtt, ObjectMapper json) {
        this.mqtt = mqtt;
        this.json = json;
    }

    /**
     * Publishes {@code payload} to {@code topic} and waits up to {@code timeout} for the broker ack.
     *
     * @return true if the broker acknowledged in time; false on timeout
     */
    public boolean publish(String topic, Object payload, Duration timeout) {
        try {
            mqtt.publishWith()
                    .topic(topic)
                    .qos(MqttQos.AT_LEAST_ONCE)
                    .payload(json.writeValueAsBytes(payload))
                    .send()
                    .get(timeout.toMillis(), TimeUnit.MILLISECONDS);
            return true;
        } catch (TimeoutException e) {
            return false;
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            return false;
        } catch (ExecutionException | JsonProcessingException e) {
            throw new IllegalStateException("MQTT publish to " + topic + " failed", e);
        }
    }

    /** Publishes and blocks until the broker acknowledges. Used for climate, where the ack can be slow. */
    public void publishAndWait(String topic, Object payload) {
        try {
            mqtt.publishWith()
                    .topic(topic)
                    .qos(MqttQos.AT_LEAST_ONCE)
                    .payload(json.writeValueAsBytes(payload))
                    .send()
                    .join();
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("MQTT publish to " + topic + " failed", e);
        }
    }
}
