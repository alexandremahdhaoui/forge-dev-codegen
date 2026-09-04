use tokio::sync::Mutex;

pub struct GreetingControllerImpl {
    pub guard: Mutex<()>,
}
