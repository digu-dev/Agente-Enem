package dev.enem.api;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

/** Serves the process liveness endpoint. */
@RestController
public class HealthController {

  @GetMapping("/health")
  public HealthResponse health() {
    return new HealthResponse("ok");
  }

  /** Health state returned by the endpoint. */
  public record HealthResponse(String status) {}
}
