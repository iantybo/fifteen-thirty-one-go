package com.northstar.vehicle.config;

import com.hivemq.client.mqtt.MqttClient;
import com.hivemq.client.mqtt.mqtt5.Mqtt5AsyncClient;
import java.time.Clock;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import software.amazon.awssdk.services.dynamodb.DynamoDbClient;

/** AWS, MQTT, and clock beans. Credentials come from the task role; nothing is configured in code. */
@Configuration
public class InfrastructureConfig {
    @Bean
    Clock clock() {
        return Clock.systemUTC();
    }

    @Bean
    DynamoDbClient dynamoDbClient() {
        return DynamoDbClient.create();
    }

    @Bean(destroyMethod = "disconnect")
    Mqtt5AsyncClient mqttClient(@Value("${northstar.mqtt.host}") String host,
            @Value("${northstar.mqtt.port:8883}") int port) {
        Mqtt5AsyncClient client = MqttClient.builder()
                .useMqttVersion5()
                .identifier("vehicle-platform-" + java.util.UUID.randomUUID())
                .serverHost(host)
                .serverPort(port)
                .sslWithDefaultConfig()
                .buildAsync();
        client.connect().join();
        return client;
    }
}
