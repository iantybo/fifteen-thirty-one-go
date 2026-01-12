package com.cribbagegame.dailychallenge;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
public class DailyChallengeServiceApplication {

    public static void main(String[] args) {
        SpringApplication.run(DailyChallengeServiceApplication.class, args);
    }
}
