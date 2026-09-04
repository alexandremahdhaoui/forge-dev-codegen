use crate::controller::zz_generated_greeting_controller::GreetingController;

pub struct Orphan;

impl GreetingController for Orphan {
    fn greet(&self, name: &str) -> String {
        format!("hello {name}")
    }
}
