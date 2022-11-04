# This image is provided by the Cypress team and contains the Cypress core and all the supported browsers.
FROM cypress/included:10.11.0

# Copy our test sources
COPY . .

# Run the tests as a one-shot command
ENTRYPOINT [ "cypress", "run" ]
