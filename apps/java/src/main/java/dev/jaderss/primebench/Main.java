package dev.jaderss.primebench;

import java.io.IOException;
import java.util.concurrent.ExecutionException;

public final class Main {

    private Main() {}

    public static void main(String[] args) throws IOException, InterruptedException, ExecutionException {
        Config config = Config.fromEnv();
        switch (config.mode()) {
            case "server" -> PrimeServer.run(config);
            case "batch" -> BatchRunner.run(config);
            default ->
                throw new IllegalArgumentException(
                        "unknown MODE %s, expected server or batch".formatted(config.mode()));
        }
    }
}
