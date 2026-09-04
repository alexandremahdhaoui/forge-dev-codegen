use crate::controller::zz_generated_greeting_controller::{GreetingController, GreetingControllerImpl};
use crate::types::greeting::Greeting;

impl GreetingController for GreetingControllerImpl {
    fn greet(&self, name: &str) -> Greeting {
        Greeting {
            text: format!("hello {name}"),
        }
    }
}
