use crate::port::greeting_store::GreetingStore;

pub struct GreetingControllerImpl {
    pub store: std::sync::Arc<dyn GreetingStore + Send + Sync>,
}
